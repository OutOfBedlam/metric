package metric

import "fmt"

type Deriver interface {
	ID() string
	WindowSize() int
	Derive(values []Value) Value
}

var _ Deriver = MovingAverage{}

type MovingAverage struct {
	// Number of points to include in the moving average calculation.
	// Must be less than or equal to the maxCount of the TimeSeries.
	// If greater than maxCount, it will be set to maxCount.
	windowSize int
}

func (ma MovingAverage) ID() string {
	return fmt.Sprintf("ma%d", ma.windowSize)
}

func (ma MovingAverage) WindowSize() int {
	return ma.windowSize
}

func (ma MovingAverage) Derive(values []Value) Value {
	var sum float64
	var samples int64
	var count int
	for _, value := range values {
		if value == nil {
			continue
		}
		switch val := value.(type) {
		case *CounterValue:
			if val.Samples == 0 {
				continue
			}
			samples += val.Samples
			sum += val.Value
			count++
		}
	}
	switch values[len(values)-1].(type) {
	case *CounterValue:
		ret := &CounterValue{
			Samples: samples,
		}
		if count > 0 {
			ret.Value = sum / float64(count)
		}
		return ret
	default:
		return nil
	}
}
