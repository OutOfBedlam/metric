package metric

import (
	"expvar"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ExampleCounter() {
	c := &Counter{}
	c.Mark(10)
	fmt.Println(c.String())

	c.Mark(5)
	fmt.Println(c.String())

	c.Reset()
	fmt.Println(c.String())

	// Output:
	//
	// 10
	// 15
	// 0
}

func ExampleCounter_expvar() {
	c := &Counter{}
	// Simulate publishing the counter value
	name := "example.counter.publish"
	expvar.Publish(name, c)

	c.Mark(20)

	expvar.Do(func(kv expvar.KeyValue) {
		if kv.Key == name {
			fmt.Println(kv.Key, kv.Value.String())
		}
	})
	// Output:
	//
	// example.counter.publish 20
}

func TestTimeSeriesCounter(t *testing.T) {
	c := &Counter{}
	ts := NewTimeSeries[float64](1*time.Second, 10, LAST)
	c.Extend(ts)

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		c.Mark(int64(i))
	}

	require.Equal(t, `[`+
		`{"ts":"2025-07-21T17:31:13+09:00","value":55.000000},`+
		`{"ts":"2025-07-21T17:31:14+09:00","value":210.000000},`+
		`{"ts":"2025-07-21T17:31:15+09:00","value":465.000000},`+
		`{"ts":"2025-07-21T17:31:16+09:00","value":820.000000},`+
		`{"ts":"2025-07-21T17:31:17+09:00","value":1275.000000},`+
		`{"ts":"2025-07-21T17:31:18+09:00","value":1830.000000},`+
		`{"ts":"2025-07-21T17:31:19+09:00","value":2485.000000},`+
		`{"ts":"2025-07-21T17:31:20+09:00","value":3240.000000},`+
		`{"ts":"2025-07-21T17:31:21+09:00","value":4095.000000},`+
		`{"ts":"2025-07-21T17:31:22+09:00","value":5050.000000}`+
		`]`, ts.String())
}

func TestHistogramCounter(t *testing.T) {
	h := NewHistogram(100)
	c := &Counter{}
	c.Extend(h)
	c.Mark(5)
	require.Equal(t, int64(5), c.Value(), "Expected counter value to be 5.5, got %d", c.Value())

	c.Mark(3)
	require.Equal(t, int64(8), c.Value(), "Expected counter value to be 8, got %d", c.Value())

	c.Reset()
	require.Equal(t, int64(0), c.Value(), "Expected counter value to be reset to 0, got %d", c.Value())

	for i := 1; i <= 10; i++ {
		c.Mark(int64(i))
	}
	require.Equal(t, int64(55), c.Value(), "Expected counter value to be 55, got %d", c.Value())
	require.Equal(t, 10.0, h.Quantile(0.5), "Expected histogram quantile 0.5 to be 5.0, got %f", h.Quantile(0.5))
	require.Equal(t, 45.0, h.Quantile(0.9), "Expected histogram quantile 0.9 to be 9.0, got %f", h.Quantile(0.9))

	for range 2000 {
		v := rand.Int64N(100)
		c.Mark(v)
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
