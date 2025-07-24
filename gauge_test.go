package metric

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ExampleGauge() {
	g := &Gauge{}
	g.Mark(42.1)
	fmt.Println(g.String())
	g.Mark(3.1415)
	fmt.Println(g.String())
	g.Reset()
	fmt.Println(g.String())

	// Output:
	//
	// 42.1
	// 3.1415
	// 0
}

func TestTimeSeriesGauge(t *testing.T) {
	ts := NewTimeSeries[float64](time.Second, 10, LAST)
	gauge := &Gauge{}
	gauge.Extend(ts)

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		gauge.Mark(float64(i))
	}

	require.Equal(t, `[`+
		`{"ts":"2025-07-21T17:31:13+09:00","value":10.000000},`+
		`{"ts":"2025-07-21T17:31:14+09:00","value":20.000000},`+
		`{"ts":"2025-07-21T17:31:15+09:00","value":30.000000},`+
		`{"ts":"2025-07-21T17:31:16+09:00","value":40.000000},`+
		`{"ts":"2025-07-21T17:31:17+09:00","value":50.000000},`+
		`{"ts":"2025-07-21T17:31:18+09:00","value":60.000000},`+
		`{"ts":"2025-07-21T17:31:19+09:00","value":70.000000},`+
		`{"ts":"2025-07-21T17:31:20+09:00","value":80.000000},`+
		`{"ts":"2025-07-21T17:31:21+09:00","value":90.000000},`+
		`{"ts":"2025-07-21T17:31:22+09:00","value":100.000000}`+
		`]`, ts.String())
}

func TestHistogramGauge(t *testing.T) {
	h := NewHistogram(100)
	g := &Gauge{}
	g.Extend(h)

	g.Mark(5.5)
	if g.Value() != 5.5 {
		t.Errorf("Expected gauge value to be 5.5, got %f", g.Value())
	}

	if h.Quantile(0.5) != 5.5 {
		t.Errorf("Expected histogram quantile 0.5 to be 10.0, got %f", h.Quantile(0.5))
	}

	g.Reset()
	if g.Value() != 0 {
		t.Errorf("Expected gauge value to be reset to 0, got %f", g.Value())
	}

	for range 2000 {
		v := rand.Float64()
		g.Mark(100.0 * v)
	}
	if q := h.Quantiles(0.5, 0.99); len(q) != 2 {
		t.Errorf("Expected 2 quantiles, got %d", len(q))
	} else {
		p50 := q[0]
		p99 := q[1]
		if p50 <= 49 && p50 >= 51 {
			t.Errorf("Expected p50 to be around 50, got %f", p50)
		}
		if p99 <= 99 && p99 >= 101 {
			t.Errorf("Expected p99 to be around 100, got %f", p99)
		}
	}
}
