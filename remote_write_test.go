package remotewrite

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
)

/*
func TestEvaluateTemplate(t *testing.T) {
	require.Equal(t, compileTemplate("something ${series_id} else")(12), "something 12 else")
	require.Equal(t, compileTemplate("something ${series_id/6} else")(12), "something 2 else")
}
*/

func TestGenerateFromTemplates(t *testing.T) {
	type args struct {
		minValue       int
		maxValue       int
		timestamp      int64
		minSeriesID    int
		maxSeriesID    int
		labelsTemplate map[string]string
	}
	type want struct {
		valueMin float64
		valueMax float64
		series   []prompb.TimeSeries
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "11th batch of 5",
			args: args{
				minValue:    123,
				maxValue:    133,
				timestamp:   123456789,
				minSeriesID: 50,
				maxSeriesID: 55,
				labelsTemplate: map[string]string{
					"__name__":        "k6_generated_metric_${series_id}",
					"series_id":       "${series_id}",
					"cardinality_1e1": "${series_id/10}",
					"cardinality_1e3": "${series_id/1000}",
				},
			},
			want: want{
				valueMin: 123,
				valueMax: 133,
				series: []prompb.TimeSeries{
					{
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_50"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "series_id", Value: "50"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_51"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "series_id", Value: "51"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_52"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "series_id", Value: "52"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_53"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "series_id", Value: "53"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					}, {
						Labels: []prompb.Label{
							{Name: "__name__", Value: "k6_generated_metric_54"},
							{Name: "cardinality_1e1", Value: "5"},
							{Name: "cardinality_1e3", Value: "0"},
							{Name: "series_id", Value: "54"},
						},
						Samples: []prompb.Sample{{Timestamp: 123456789}},
					},
				},
			},
		},
	}
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := precompileLabelTemplates(tt.args.labelsTemplate)
			got := generateFromTemplates(r, tt.args.minValue, tt.args.maxValue, tt.args.timestamp, tt.args.minSeriesID, tt.args.maxSeriesID, compiled)
			if len(got) != len(tt.want.series) {
				t.Errorf("Differing length, want: %d, got: %d", len(tt.want.series), len(got))
			}

			for seriesId := range got {
				if !reflect.DeepEqual(got[seriesId].Labels, tt.want.series[seriesId].Labels) {
					t.Errorf("Unexpected labels in series %d, want: %v, got: %v", seriesId, tt.want.series[seriesId].Labels, got[seriesId].Labels)
				}

				if got[seriesId].Samples[0].Timestamp != tt.want.series[seriesId].Samples[0].Timestamp {
					t.Errorf("Unexpected timestamp in series %d, want: %d, got: %d", seriesId, tt.want.series[seriesId].Samples[0].Timestamp, got[seriesId].Samples[0].Timestamp)
				}

				if got[seriesId].Samples[0].Value < tt.want.valueMin || got[seriesId].Samples[0].Value > tt.want.valueMax {
					t.Errorf("Unexpected value in series %d, want: %f-%f, got: %f", seriesId, tt.want.valueMin, tt.want.valueMax, got[seriesId].Samples[0].Value)
				}
			}
		})
	}
}

// this test that the prompb stream marshalling implementation produces the same result as the upstream one
func TestStreamEncoding(t *testing.T) {
	seed := time.Now().Unix()
	t.Logf("seed=%d", seed)
	r := rand.New(rand.NewSource(seed))
	timestamp := int64(valueBetween(r, 10, 100)) // timestamp
	r = rand.New(rand.NewSource(seed))           // reset
	minValue := 10
	maxValue := 100000
	// this is the upstream encoding. It is purposefully this "handwritten"
	d, _ := proto.Marshal(&prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: (timestamp),
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 7 thing"},
					{Name: "forth", Value: "some 15 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 8 thing"},
					{Name: "forth", Value: "some 16 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 0 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 8 thing"},
					{Name: "forth", Value: "some 17 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 9 thing"},
					{Name: "forth", Value: "some 18 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 0 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 9 thing"},
					{Name: "forth", Value: "some 19 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 1 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 10 thing"},
					{Name: "forth", Value: "some 20 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 2 thing"},
					{Name: "third", Value: "some 0 thing"},
				},
			},
			{
				Samples: []prompb.Sample{{
					Value:     valueBetween(r, minValue, maxValue),
					Timestamp: timestamp,
				}},
				Labels: []prompb.Label{
					{Name: "fifth", Value: "some 10 thing"},
					{Name: "forth", Value: "some 21 thing"},
					{Name: "here", Value: "else"},
					{Name: "here2", Value: "else2"},
					{Name: "sixth", Value: "some 2 thing"},
					{Name: "third", Value: "some 1 thing"},
				},
			},
		},
	})

	r = rand.New(rand.NewSource(seed)) // reset
	template := precompileLabelTemplates(map[string]string{
		"here":  "else",
		"here2": "else2",
		"third": "some ${series_id%2} thing",
		"forth": "some ${series_id} thing",
		"fifth": "some ${series_id/2} thing",
		"sixth": "some ${series_id/10} thing",
	})

	buf := generateFromPrecompiledTemplates(r, minValue, maxValue, timestamp, 15, 22, template)
	b := buf.Bytes()
	require.Equal(t, d, b)
}

func BenchmarkWriteFor(b *testing.B) {
	tsBuf := new(bytes.Buffer)
	template := precompileLabelTemplates(map[string]string{
		"__name__":        "k6_generated_metric_${series_id/1000}", // Name of the series.
		"series_id":       "${series_id}",                          // Each value of this label will match 1 series.
		"cardinality_1e1": "${series_id/10}",                       // Each value of this label will match 10 series.
		"cardinality_1e2": "${series_id/100}",                      // Each value of this label will match 100 series.
		"cardinality_1e3": "${series_id/1000}",                     // Each value of this label will match 1000 series.
		"cardinality_1e4": "${series_id/10000}",                    // Each value of this label will match 10000 series.
		"cardinality_1e5": "${series_id/100000}",                   // Each value of this label will match 100000 series.
		"cardinality_1e6": "${series_id/1000000}",                  // Each value of this label will match 1000000 series.
		"cardinality_1e7": "${series_id/10000000}",                 // Each value of this label will match 10000000 series.
		"cardinality_1e8": "${series_id/100000000}",                // Each value of this label will match 100000000 series.
		"cardinality_1e9": "${series_id/1000000000}",               // Each value of this label will match 1000000000 series.
	})
	template.writeFor(tsBuf, 15, 15, 234)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		template.writeFor(tsBuf, 15, i, 234)
	}
}
