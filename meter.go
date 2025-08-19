package metric

import (
	"encoding/json"
	"sync"
)

func NewMeter() *Meter {
	return &Meter{}
}

var _ Producer = (*Meter)(nil)

type Meter struct {
	sync.Mutex
	first float64
	last  float64
	min   float64
	max   float64
	sum   float64
	count float64
}

func (m *Meter) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Produce(false))
}

func (m *Meter) UnmarshalJSON(data []byte) error {
	p := &MeterProduct{}
	if err := json.Unmarshal(data, p); err != nil {
		return err
	}
	m.first = p.First
	m.last = p.Last
	m.min = p.Min
	m.max = p.Max
	m.sum = p.Sum
	m.count = float64(p.Count)
	return nil
}

func (m *Meter) Add(v float64) {
	m.Lock()
	defer m.Unlock()
	if m.count == 0 {
		m.first = v
		m.min = v
		m.max = v
	}
	if v < m.min {
		m.min = v
	}
	if v > m.max {
		m.max = v
	}
	m.sum += v
	m.last = v
	m.count++
}

func (m *Meter) Produce(reset bool) Product {
	m.Lock()
	defer m.Unlock()
	ret := &MeterProduct{
		Count: int64(m.count),
		First: float64(m.first),
		Last:  float64(m.last),
		Min:   float64(m.min),
		Max:   float64(m.max),
		Sum:   float64(m.sum),
	}
	if reset {
		m.first = 0
		m.last = 0
		m.min = 0
		m.max = 0
		m.sum = 0
		m.count = 0
	}
	return ret
}

func (m *Meter) String() string {
	b, _ := json.Marshal(m.Produce(false))
	return string(b)
}

type MeterProduct struct {
	Count int64   `json:"count"`
	Sum   float64 `json:"sum"`
	First float64 `json:"first"`
	Last  float64 `json:"last"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

func (mp *MeterProduct) String() string {
	b, _ := json.Marshal(mp)
	return string(b)
}
