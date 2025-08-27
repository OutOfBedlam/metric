package metric

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetric(t *testing.T) {
	now := time.Unix(1756251515, 0)
	nowFunc = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	var wg sync.WaitGroup
	var out string
	wg.Add(1)
	c := NewCollector(
		WithCollectInterval(time.Second),
		WithSeriesListener("1m/1s", time.Second, 60, func(tb TimeBin, fi FieldInfo) {
			out = fmt.Sprintf("TimeBin: %v, FieldInfo: %s:%s", tb, fi.Measure, fi.Name)
			wg.Done()
		}),
	)
	c.AddInputFunc(func() (Measurement, error) {
		m := Measurement{Name: "m1"}
		m.AddField(Field{Name: "f1", Value: 1.0, Unit: UnitShort, Type: FieldTypeCounter})
		return m, nil
	})
	c.Start()
	wg.Wait()
	require.Equal(t, `TimeBin: {"ts":"2025-08-27 08:38:37","value":{"samples":1,"value":1}}, FieldInfo: m1:f1`, out)
	c.Stop()
}
