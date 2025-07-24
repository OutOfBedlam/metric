package metric

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ExampleTimer() {
	timer := &Timer{}

	// Simulate some work
	timer.Mark(1100 * time.Millisecond)

	timer.Mark(400 * time.Millisecond)

	fmt.Println("Avg:", timer.String())

	s := timer.Snapshot()
	fmt.Printf("%+v\n", s)

	// Output:
	//
	// Avg: 750ms
	// {Count:2 TotalDuration:1.5s MinDuration:400ms MaxDuration:1.1s}
}

func TestTimer(t *testing.T) {
	hist := NewHistogram(1000)
	timer := &Timer{}
	timer.Extend(TimerExtendValue, hist)

	timer.Mark(10 * time.Millisecond)

	timer.Mark(20 * time.Millisecond)

	for i := 3; i <= 100; i++ {
		timer.Mark(time.Duration(i*10) * time.Millisecond)
	}
	require.Equal(t, timer.totalDuration, 50500*time.Millisecond)
	require.Equal(t, timer.count, int64(100))
	require.Equal(t, 10*time.Millisecond, timer.minDuration)
	require.Equal(t, 1000*time.Millisecond, timer.maxDuration)
	require.Equal(t, `505ms`, timer.String())

	var p50, p75, p90, p99, p999 time.Duration
	if h := hist.Quantiles(0.5, 0.75, 0.9, 0.99, 0.999); len(h) == 5 {
		p50 = time.Duration(h[0])
		p75 = time.Duration(h[1])
		p90 = time.Duration(h[2])
		p99 = time.Duration(h[3])
		p999 = time.Duration(h[4])
	} else {
		t.Fatal("Failed to get histogram quantiles")
	}
	require.Equal(t, 500*time.Millisecond, p50)
	require.Equal(t, 750*time.Millisecond, p75)
	require.Equal(t, 900*time.Millisecond, p90)
	require.Equal(t, 990*time.Millisecond, p99)
	require.Equal(t, 1000*time.Millisecond, p999)
}

func TestTimeSeriesTimer(t *testing.T) {
	ts := NewTimeSeries[float64](time.Second, 10, LAST)
	timer := &Timer{}
	timer.Extend(TimerExtendAvg, ts)

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		timer.Mark(time.Duration(i * int(time.Millisecond)))
	}

	require.Equal(t, `[`+
		`{"ts":"2025-07-21T17:31:13+09:00","value":5500000.000000},`+
		`{"ts":"2025-07-21T17:31:14+09:00","value":10500000.000000},`+
		`{"ts":"2025-07-21T17:31:15+09:00","value":15500000.000000},`+
		`{"ts":"2025-07-21T17:31:16+09:00","value":20500000.000000},`+
		`{"ts":"2025-07-21T17:31:17+09:00","value":25500000.000000},`+
		`{"ts":"2025-07-21T17:31:18+09:00","value":30500000.000000},`+
		`{"ts":"2025-07-21T17:31:19+09:00","value":35500000.000000},`+
		`{"ts":"2025-07-21T17:31:20+09:00","value":40500000.000000},`+
		`{"ts":"2025-07-21T17:31:21+09:00","value":45500000.000000},`+
		`{"ts":"2025-07-21T17:31:22+09:00","value":50500000.000000}`+
		`]`, ts.String())
}

func TestHistogramTimer(t *testing.T) {
	h := NewHistogram(1000)
	timer := &Timer{}
	timer.Extend(TimerExtendValue, h)

	for i := 1; i <= 100; i++ {
		timer.Mark(time.Duration(i*10) * time.Millisecond)
	}

	require.Equal(t, `505ms`, timer.String())
	require.Equal(t, int64(100), timer.count)
	require.Equal(t, 50500*time.Millisecond, timer.totalDuration)

	var p50, p75, p90, p99, p999 time.Duration
	if h := h.Quantiles(0.5, 0.75, 0.9, 0.99, 0.999); len(h) == 5 {
		p50 = time.Duration(h[0])
		p75 = time.Duration(h[1])
		p90 = time.Duration(h[2])
		p99 = time.Duration(h[3])
		p999 = time.Duration(h[4])
	} else {
		t.Fatal("Failed to get histogram quantiles")
	}
	require.Equal(t, 500*time.Millisecond, p50)
	require.Equal(t, 750*time.Millisecond, p75)
	require.Equal(t, 900*time.Millisecond, p90)
	require.Equal(t, 990*time.Millisecond, p99)
	require.Equal(t, 1000*time.Millisecond, p999)
}
