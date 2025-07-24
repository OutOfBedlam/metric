package metric

import (
	"strconv"
	"sync"
)

var _ Value[float64] = (*Meter)(nil)

type MeterExtendPoint int

const (
	MeterExtendValue MeterExtendPoint = iota
	MeterExtendMin
	MeterExtendMax
	MeterExtendSum
	MeterExtendCount
	MeterExtendAvg
)

type Meter struct {
	sync.Mutex
	value float64
	min   float64
	max   float64
	sum   float64
	count int64
	exts  map[MeterExtendPoint]Extension
}

func (m *Meter) Extend(point MeterExtendPoint, ext Extension) {
	m.Lock()
	defer m.Unlock()
	if m.exts == nil {
		m.exts = make(map[MeterExtendPoint]Extension)
	}
	if ext == nil {
		delete(m.exts, point)
		return
	}
	m.exts[point] = ext
}

func (m *Meter) String() string {
	return strconv.FormatFloat(m.Value(), 'g', -1, 64)
}

func (m *Meter) Value() float64 {
	m.Lock()
	defer m.Unlock()
	return m.value
}

type MeterSnapshot struct {
	Value float64 `json:"value"`
	Count int64   `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

func (m *Meter) Snapshot() MeterSnapshot {
	m.Lock()
	defer m.Unlock()
	return MeterSnapshot{
		Value: m.value,
		Count: m.count,
		Sum:   m.sum,
		Min:   m.min,
		Max:   m.max,
	}
}

func (m *Meter) Mark(v float64) {
	m.Lock()
	defer m.Unlock()
	m.value = v
	m.sum += v
	m.count++
	if m.count == 1 || v < m.min {
		m.min = v
	}
	if v > m.max {
		m.max = v
	}
	for point, ext := range m.exts {
		switch point {
		case MeterExtendValue:
			ext.Add(v)
		case MeterExtendMin:
			ext.Add(m.min)
		case MeterExtendMax:
			ext.Add(m.max)
		case MeterExtendSum:
			ext.Add(m.sum)
		case MeterExtendCount:
			ext.Add(float64(m.count))
		case MeterExtendAvg:
			ext.Add(m.sum / float64(m.count))
		default:
			panic("unknown meter adder point: " + strconv.Itoa(int(point)))
		}
	}
}

func (m *Meter) Reset() {
	m.Lock()
	defer m.Unlock()
	m.value = 0
	m.min = 0
	m.max = 0
	m.sum = 0
	m.count = 0
}
