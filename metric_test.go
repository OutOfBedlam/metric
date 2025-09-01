package metric

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetric(t *testing.T) {
	var wg sync.WaitGroup
	var out string
	var cnt int
	var now time.Time
	wg.Add(3)
	c := NewCollector(
		WithCollectInterval(time.Second),
		WithSeriesListener("1m/1s", time.Second, 60, func(pd ProducedData) {
			defer wg.Done()
			out = fmt.Sprintf("%s:%s %s %v %s %s",
				pd.Measure, pd.Field, pd.Series, pd.Time.Format(time.TimeOnly), pd.Value.String(), pd.Type)
			if cnt == 0 {
				now = pd.Time
			} else {
				now = now.Add(time.Second)
			}
			cnt++
			expect := fmt.Sprintf(`m1:f1 1m/1s %s {"samples":1,"value":1} counter`, now.Format(time.TimeOnly))
			require.Equal(t, expect, out)
		}),
	)
	c.AddInputFunc(func() (Measurement, error) {
		m := Measurement{Name: "m1"}
		m.AddField(Field{Name: "f1", Value: 1.0, Type: CounterType(UnitShort)})
		return m, nil
	})
	c.Start()
	wg.Wait()
	c.Stop()
}
