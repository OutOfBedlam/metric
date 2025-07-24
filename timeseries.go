package metric

import (
	"fmt"
	"sync"
	"time"
)

type TimePoint[T any] struct {
	Time  time.Time
	Value T
	Count int // If Count is 0, it means the point is empty
}

type TimeSeries[T any] struct {
	sync.Mutex
	data       []TimePoint[T]
	interval   time.Duration
	maxCount   int
	aggregator Aggregator[T]
	useRawTime bool
	meta       *TimeSeriesMeta // Optional metadata for the time series
}

type TimeSeriesMeta struct {
	Title string
	Unit  string
}

// If aggregator is nil, it will replace the last point with the new one.
// Otherwise, it will aggregate the new point with the last one when it falls within the same interval.
func NewTimeSeries[T any](interval time.Duration, maxCount int, agg Aggregator[T]) *TimeSeries[T] {
	return &TimeSeries[T]{
		data:       make([]TimePoint[T], 0, maxCount),
		interval:   interval,
		maxCount:   maxCount,
		aggregator: agg,
	}
}

func (ts *TimeSeries[T]) roundTime(t time.Time) time.Time {
	if ts.useRawTime {
		return t
	}
	return t.Add(ts.interval / 2).Round(ts.interval)
}

func (ts *TimeSeries[T]) SetMeta(meta *TimeSeriesMeta) {
	ts.meta = meta
}

func (ts *TimeSeries[T]) Meta() *TimeSeriesMeta {
	return ts.meta
}

func (ts *TimeSeries[T]) String() string {
	ts.Lock()
	defer ts.Unlock()
	result := "["
	for i, d := range ts.data {
		if i > 0 {
			result += ","
		}
		t := d.Time.Format(time.RFC3339)
		if i == len(ts.data)-1 {
			// Round the last time
			t = ts.roundTime(d.Time).Format(time.RFC3339)
		}
		v := ""
		switch val := (any(d.Value)).(type) {
		case float64:
			v = fmt.Sprintf("%f", val)
		case int64:
			v = fmt.Sprintf("%d", val)
		default:
			v = fmt.Sprintf("%v", val)
		}
		if d.Count == 0 {
			result += fmt.Sprintf(`{"ts":"%s"}`, t)
		} else {
			result += fmt.Sprintf(`{"ts":"%s","value":%s}`, t, v)
		}
	}
	result += "]"
	return result
}

type TimeSeriesSnapshot[T any] struct {
	Times    []time.Time
	Values   []T
	Interval time.Duration
	MaxCount int
}

// Snapshot creates a snapshot of the current time series data.
// If snapshot is nil, it will create a new one.
// The snapshot will contain rounded times and values.
func (ts *TimeSeries[T]) Snapshot(snapshot *TimeSeriesSnapshot[T]) *TimeSeriesSnapshot[T] {
	ts.Lock()
	defer ts.Unlock()
	if snapshot == nil {
		snapshot = &TimeSeriesSnapshot[T]{}
	}
	if len(snapshot.Times) != len(ts.data) {
		snapshot.Times = make([]time.Time, len(ts.data))
	}
	if len(snapshot.Values) != len(ts.data) {
		snapshot.Values = make([]T, len(ts.data))
	}
	snapshot.Interval = ts.interval
	snapshot.MaxCount = ts.maxCount
	for i, d := range ts.data {
		if i == len(ts.data)-1 {
			// Round the last time
			snapshot.Times[i] = ts.roundTime(d.Time)
		} else {
			snapshot.Times[i] = d.Time
		}
		snapshot.Values[i] = d.Value
	}
	return snapshot
}

func (ts *TimeSeries[T]) Values() ([]time.Time, []T) {
	ts.Lock()
	defer ts.Unlock()
	times := make([]time.Time, len(ts.data))
	values := make([]T, len(ts.data))
	for i, d := range ts.data {
		times[i] = d.Time
		values[i] = d.Value
	}
	times[len(times)-1] = ts.roundTime(times[len(times)-1]) // Round the last time
	return times, values
}

func (ts *TimeSeries[T]) Last() (time.Time, T) {
	times, values := ts.LastN(1)
	if len(times) == 0 {
		return time.Time{}, *new(T)
	}
	return times[0], values[0]
}

func (ts *TimeSeries[T]) LastN(n int) ([]time.Time, []T) {
	ts.Lock()
	defer ts.Unlock()
	if len(ts.data) == 0 || n <= 0 {
		return nil, nil
	}
	offset := len(ts.data) - n
	if offset < 0 {
		offset = 0
	}

	sub := ts.data[offset:]
	times := make([]time.Time, len(sub))
	values := make([]T, len(sub))
	for i := range sub {
		times[i], values[i] = sub[i].Time, sub[i].Value
	}
	times[len(times)-1] = ts.roundTime(times[len(times)-1]) // Round the last time
	return times, values
}

func (ts *TimeSeries[T]) After(t time.Time) ([]time.Time, []T) {
	ts.Lock()
	defer ts.Unlock()
	idx := -1
	tick := t.UnixNano() - (int64(ts.interval) / 2)
	for i, d := range ts.data {
		if d.Time.UnixNano() >= tick {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, nil
	}
	sub := ts.data[idx:]
	times := make([]time.Time, len(sub))
	values := make([]T, len(sub))
	for i := range sub {
		times[i], values[i] = sub[i].Time, sub[i].Value
	}
	times[len(times)-1] = ts.roundTime(times[len(times)-1]) // Round the last time
	return times, values
}

func (ts *TimeSeries[T]) Add(v T) {
	ts.Lock()
	defer ts.Unlock()
	ts.addPoint(TimePoint[T]{Time: nowFunc(), Value: v, Count: 1})
}

func (ts *TimeSeries[T]) AddTime(t time.Time, v T) {
	ts.Lock()
	defer ts.Unlock()
	ts.addPoint(TimePoint[T]{Time: t, Value: v, Count: 1})
}

func (ts *TimeSeries[T]) addPoint(datum TimePoint[T]) {
	if len(ts.data) == 0 {
		ts.data = append(ts.data, datum)
		return
	}
	last := ts.data[len(ts.data)-1]

	roll := ts.IntervalBetween(last.Time, datum.Time)

	if roll <= 0 {
		if ts.aggregator == nil {
			// Replace the last point
			ts.data[len(ts.data)-1] = datum
		} else {
			// If the new datum is within the same interval, aggregate it
			ts.data[len(ts.data)-1] = ts.aggregator(last, datum)
		}
		return
	}

	if roll >= ts.maxCount {
		ts.data = ts.data[:0] // Reset if the gap is too large
		ts.data = append(ts.data, datum)
		return
	}

	for i := range roll {
		// Remove the oldest data if we exceed maxCount
		if len(ts.data) >= ts.maxCount {
			ts.data = ts.data[len(ts.data)-ts.maxCount+1:]
		}
		if i == roll-1 {
			ts.data[len(ts.data)-1].Time = ts.roundTime(ts.data[len(ts.data)-1].Time)
			// Add the new datum at the end of the series
			ts.data = append(ts.data, datum)
		} else {
			// Fill in the gaps with empty data points
			emptyPoint := TimePoint[T]{
				Time: last.Time.Add(time.Duration(i+1) * ts.interval),
			}
			ts.data = append(ts.data, emptyPoint)
		}
	}
}

// IntervalBetween returns the number of intervals between two times.
// (later - prev) / ts.interval
func (ts *TimeSeries[T]) IntervalBetween(prev, later time.Time) int {
	return int(ts.timeRound(later).Sub(ts.timeRound(prev)) / ts.interval)
}

// IntervalFromLast returns the number of intervals between the last recorded time and a given time.
func (ts *TimeSeries[T]) IntervalFromLast(t time.Time) int {
	if len(ts.data) == 0 {
		return 0
	}
	last := ts.data[len(ts.data)-1].Time
	return ts.IntervalBetween(last, t)
}

func (ts *TimeSeries[T]) timeRound(t time.Time) time.Time {
	return t.Truncate(ts.interval)
}

type MultiTimeSeries[T any] []*TimeSeries[T]

func (mts MultiTimeSeries[T]) Add(v T) {
	for _, ts := range mts {
		ts.Add(v)
	}
}

func (mts MultiTimeSeries[T]) AddTime(t time.Time, v T) {
	for _, ts := range mts {
		ts.AddTime(t, v)
	}
}

func (mts *MultiTimeSeries[T]) SetMeta(meta *TimeSeriesMeta) {
	for _, ts := range *mts {
		ts.SetMeta(meta)
	}
}

func (mts MultiTimeSeries[T]) String() string {
	if len(mts) == 0 {
		return "[]"
	}
	result := "["
	for i, ts := range mts {
		if i > 0 {
			result += ","
		}
		result += ts.String()
	}
	result += "]"
	return result
}

type Aggregator[T any] func(l, r TimePoint[T]) TimePoint[T]

//////////////////////////////////////////////////////////
// Some common aggregators
// SUM, AVG, LAST, FIRST, MIN, MAX

func SUM[T ValueType](l, r TimePoint[T]) TimePoint[T] {
	ts := r.Time
	if l.Time.After(r.Time) { // Ensure l is after r
		ts = l.Time
	}
	return TimePoint[T]{
		Time:  ts,
		Value: l.Value + r.Value,
		Count: l.Count + r.Count,
	}
}

func AVG[T ValueType](l, r TimePoint[T]) TimePoint[T] {
	ts := r.Time
	if l.Time.After(r.Time) { // Ensure l is after r
		ts = l.Time
	}
	totalCount := l.Count + r.Count
	if totalCount == 0 {
		return TimePoint[T]{Time: ts}
	}
	value := ((float64(l.Value) * float64(l.Count)) + (float64(r.Value) * float64(r.Count))) / float64(totalCount)
	return TimePoint[T]{
		Time:  ts,
		Value: T(value),
		Count: totalCount,
	}
}

func LAST[T any](l, r TimePoint[T]) TimePoint[T] {
	if l.Time.After(r.Time) {
		return l
	}
	return r
}

func FIRST[T any](l, r TimePoint[T]) TimePoint[T] {
	if l.Time.Before(r.Time) {
		return l
	}
	return r
}

func MIN[T ValueType](l, r TimePoint[T]) TimePoint[T] {
	if l.Value < r.Value {
		return l
	}
	return r
}

func MAX[T ValueType](l, r TimePoint[T]) TimePoint[T] {
	if l.Value > r.Value {
		return l
	}
	return r
}
