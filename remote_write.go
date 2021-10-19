package remotewrite

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/prompb"
	"github.com/xhit/go-str2duration/v2"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/stats"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/remotewrite".
func init() {
	modules.Register("k6/x/remotewrite", new(RemoteWrite))
}

// RemoteWrite is the k6 extension for interacting with Kubernetes jobs.
type RemoteWrite struct {
}

// Client is the client wrapper.
type Client struct {
	client *http.Client
	cfg    *Config
}

type Config struct {
	Url        string `json:"url"`
	UserAgent  string `json:"user_agent"`
	Timeout    string `json:"timeout"`
	TenantName string `json:"tenant_name"`
}

// XClient represents
func (r *RemoteWrite) XClient(ctxPtr *context.Context, config Config) interface{} {
	if config.Url == "" {
		log.Fatal(fmt.Errorf("url is required"))
	}
	if config.UserAgent == "" {
		config.UserAgent = "k6-remote-write/0.0.1"
	}
	if config.Timeout == "" {
		config.Timeout = "10s"
	}

	rt := common.GetRuntime(*ctxPtr)
	return common.Bind(rt, &Client{
		client: &http.Client{},
		cfg:    &config,
	}, ctxPtr)
}

type Timeseries struct {
	Labels []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"labels"`
	Samples []struct {
		Value     float64 `json:"value"`
		Timestamp int64   `json:"timestamp"`
	} `json:"samples"`
}

func (c *Client) Store(ctx context.Context, ts []Timeseries) (http.Response, error) {
	var batch []prompb.TimeSeries
	for _, t := range ts {
		batch = append(batch, FromTimeseriesToPrometheusTimeseries(t))
	}

	// Required for k6 metrics
	state := lib.GetState(ctx)
	if state == nil {
		return http.Response{}, errors.New("State is nil")
	}

	now := time.Now()
	stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
		Metric: RemoteWriteNumSeries,
		Time:   now,
		Value:  float64(len(batch)),
	})

	req := prompb.WriteRequest{
		Timeseries: batch,
	}

	data, err := proto.Marshal(&req)
	if err != nil {
		return http.Response{}, errors.Wrap(err, "failed to marshal remote-write request")
	}

	compressed := snappy.Encode(nil, data)

	res, err := c.send(ctx, state, compressed)
	if err != nil {
		return http.Response{}, errors.Wrap(err, "remote-write request failed")
	}

	return res, nil
}

// send sends a batch of samples to the HTTP endpoint, the request is the proto marshalled
// and encoded bytes
func (c *Client) send(ctx context.Context, state *lib.State, req []byte) (http.Response, error) {
	httpReq, err := http.NewRequest("POST", c.cfg.Url, bytes.NewReader(req))
	if err != nil {
		return http.Response{}, err
	}
	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("User-Agent", c.cfg.UserAgent)
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	if c.cfg.TenantName != "" {
		httpReq.Header.Set("X-Scope-OrgID", c.cfg.TenantName)
	}

	duration, err := str2duration.ParseDuration(c.cfg.Timeout)
	if err != nil {
		return http.Response{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	httpReq = httpReq.WithContext(ctx)
	now := time.Now()

	stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
		Metric: RemoteWriteReqs,
		Time:   now,
		Value:  float64(1),
	})

	simpleNetTrail := netext.NetTrail{
		BytesWritten: int64(binary.Size(req)),
		StartTime:    now.Add(-time.Minute),
		EndTime:      now,
		Samples: []stats.Sample{
			{
				Time:   now,
				Metric: metrics.DataSent,
				Value:  float64(binary.Size(req)),
			},
		},
	}
	stats.PushIfNotDone(ctx, state.Samples, &simpleNetTrail)

	start := time.Now()
	httpResp, err := c.client.Do(httpReq)
	elapsed := time.Since(start)
	if err != nil {
		return http.Response{}, err
	}

	stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
		Metric: RemoteWriteReqDuration,
		Time:   now,
		Value:  float64(elapsed.Milliseconds()),
	})

	if httpResp.StatusCode != http.StatusOK {
		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: RemoteWriteReqFailed,
			Time:   now,
			Value:  float64(1),
		})
	}

	return *httpResp, err
}

func FromTimeseriesToPrometheusTimeseries(ts Timeseries) prompb.TimeSeries {
	var labels []prompb.Label
	var samples []prompb.Sample
	for _, label := range ts.Labels {
		labels = append(labels, prompb.Label{
			Name:  label.Name,
			Value: label.Value,
		})
	}
	for _, sample := range ts.Samples {
		if sample.Timestamp == 0 {
			sample.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
		}
		samples = append(samples, prompb.Sample{
			Value:     sample.Value,
			Timestamp: sample.Timestamp,
		})
	}

	return prompb.TimeSeries{
		Labels:  labels,
		Samples: samples,
	}
}
