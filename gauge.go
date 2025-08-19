package metric

import (
	"encoding/json"
	"sync"
)

func NewGauge() *Gauge {
	return &Gauge{}
}

var _ Producer = (*Gauge)(nil)

type Gauge struct {
	sync.Mutex
	count int64
	sum   float64
	value float64
}

func (fs *Gauge) MarshalJSON() ([]byte, error) {
	p := fs.Produce(false)
	return json.Marshal(p)
}

func (fs *Gauge) UnmarshalJSON(data []byte) error {
	p := &GaugeProduct{}
	if err := json.Unmarshal(data, p); err != nil {
		return err
	}
	fs.count = p.Count
	fs.sum = p.Sum
	fs.value = p.Value
	return nil
}

func (fs *Gauge) Add(v float64) {
	fs.Lock()
	defer fs.Unlock()
	fs.value = v
	fs.sum += v
	fs.count++
}

func (fs *Gauge) Produce(reset bool) Product {
	fs.Lock()
	defer fs.Unlock()
	ret := &GaugeProduct{
		Count: int64(fs.count),
		Value: float64(fs.value),
		Sum:   float64(fs.sum),
	}
	if reset {
		fs.value = 0 // Reset the last value after peeking
		fs.count = 0 // Reset the count after peeking
		fs.sum = 0   // Reset the total after peeking
	}
	return ret
}

func (fs *Gauge) String() string {
	return fs.Produce(false).String()
}

type GaugeProduct struct {
	Count int64   `json:"count"`
	Sum   float64 `json:"sum"`
	Value float64 `json:"value"`
}

func (gp *GaugeProduct) String() string {
	b, _ := json.Marshal(gp)
	return string(b)
}
