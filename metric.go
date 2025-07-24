package metric

import (
	"time"
)

type ValueType interface {
	~float64 | ~int64
}

type Value[T ValueType] interface {
	Mark(T)
	Value() T
	Reset()
	// String returns a valid JSON value for the variable.
	// Types with String methods that do not return valid JSON
	// (such as time.Time) must not be used as a Var.
	String() string
}

var nowFunc func() time.Time = time.Now

type Extension interface {
	Add(v float64)
}

var _ Extension = (*Histogram)(nil)
var _ Extension = (*TimeSeries[float64])(nil)
