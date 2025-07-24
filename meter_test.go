package metric

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ExampleMeter() {
	m := &Meter{}
	m.Mark(42.1)
	fmt.Printf("%+v\n", m.Snapshot())
	m.Mark(3.1415)
	fmt.Printf("%+v\n", m.Snapshot())
	m.Reset()
	fmt.Printf("%+v\n", m.Snapshot())

	// Output:
	//
	// {Value:42.1 Count:1 Sum:42.1 Min:42.1 Max:42.1}
	// {Value:3.1415 Count:2 Sum:45.2415 Min:3.1415 Max:42.1}
	// {Value:0 Count:0 Sum:0 Min:0 Max:0}
}

func TestMeter(t *testing.T) {
	m := &Meter{}

	h := NewHistogram(100)
	m.Extend(MeterExtendValue, h)

	for i := 1; i <= 100; i++ {
		m.Mark(float64(i))
	}
	require.Equal(t, float64(100), m.Value(), "Expected meter value to be 100, got %f", m.Value())
	require.Equal(t, int64(100), m.count, "Expected meter count to be 100, got %d", m.count)
	require.Equal(t, float64(5050), m.sum, "Expected meter sum to be 5050, got %f", m.sum)
	require.Equal(t, float64(1), m.min, "Expected meter min to be 1, got %f", m.min)
	require.Equal(t, float64(100), m.max, "Expected meter max to be 100, got %f", m.max)
	require.Equal(t, `{Value:100 Count:100 Sum:5050 Min:1 Max:100}`, fmt.Sprintf("%+v", m.Snapshot()), "Expected meter string to match")

	var p50, p75, p90, p99, p999 float64
	if h := h.Quantiles(0.5, 0.75, 0.9, 0.99, 0.999); len(h) == 5 {
		p50 = h[0]
		p75 = h[1]
		p90 = h[2]
		p99 = h[3]
		p999 = h[4]
	} else {
		t.Fatal("Failed to get histogram quantiles")
	}

	require.Equal(t, 50.0, p50, "Expected histogram quantile 0.5 to be 50.0, got %f", p50)
	require.Equal(t, 75.0, p75, "Expected histogram quantile 0.75 to be 75.0, got %f", p75)
	require.Equal(t, 90.0, p90, "Expected histogram quantile 0.9 to be 90.0, got %f", p90)
	require.Equal(t, 99.0, p99, "Expected histogram quantile 0.99 to be 99.0, got %f", p99)
	require.Equal(t, 100.0, p999, "Expected histogram quantile 0.999 to be 100.0, got %f", p999)
}

func TestTimeSeriesMeter(t *testing.T) {
	ts := NewTimeSeries[float64](time.Second, 10, LAST)
	meter := &Meter{}
	meter.Extend(MeterExtendAvg, ts)

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		meter.Mark(float64(i))
	}

	require.Equal(t, `[`+
		`{"ts":"2025-07-21T17:31:13+09:00","value":5.500000},`+
		`{"ts":"2025-07-21T17:31:14+09:00","value":10.500000},`+
		`{"ts":"2025-07-21T17:31:15+09:00","value":15.500000},`+
		`{"ts":"2025-07-21T17:31:16+09:00","value":20.500000},`+
		`{"ts":"2025-07-21T17:31:17+09:00","value":25.500000},`+
		`{"ts":"2025-07-21T17:31:18+09:00","value":30.500000},`+
		`{"ts":"2025-07-21T17:31:19+09:00","value":35.500000},`+
		`{"ts":"2025-07-21T17:31:20+09:00","value":40.500000},`+
		`{"ts":"2025-07-21T17:31:21+09:00","value":45.500000},`+
		`{"ts":"2025-07-21T17:31:22+09:00","value":50.500000}`+
		`]`, ts.String())
}

func TestHistogramMeter(t *testing.T) {
	h := NewHistogram(100)
	m := &Meter{}
	m.Extend(MeterExtendAvg, h)

	for i := 1; i <= 100; i++ {
		m.Mark(float64(i))
	}

	require.Equal(t, float64(100), m.Value(), "Expected meter value to be 5050, got %f", m.Value())
	require.Equal(t, int64(100), m.count, "Expected meter count to be 100, got %d", m.count)
	require.Equal(t, float64(5050), m.sum, "Expected meter sum to be 5050, got %f", m.sum)
	require.Equal(t, float64(1), m.min, "Expected meter min to be 1, got %f", m.min)
	require.Equal(t, float64(100), m.max, "Expected meter max to be 100, got %f", m.max)

	if q := h.Quantiles(0.5, 0.99); len(q) != 2 {
		t.Errorf("Expected 2 quantiles, got %d", len(q))
	} else {
		p50 := q[0]
		p99 := q[1]
		require.Equal(t, 25.5, p50, "Expected histogram quantile 0.5 to be 50.0, got %f", p50)
		require.Equal(t, 50.0, p99, "Expected histogram quantile 0.99 to be 99.0, got %f", p99)
	}
}
