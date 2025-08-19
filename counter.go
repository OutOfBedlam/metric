package metric

import (
	"encoding/json"
	"sync"
)

func NewCounter() *Counter {
	return &Counter{}
}

var _ Producer = (*Counter)(nil)

type Counter struct {
	sync.Mutex
	count int64
	value float64
}

func (fs *Counter) MarshalJSON() ([]byte, error) {
	p := fs.Produce(false)
	return json.Marshal(p)
}

func (fs *Counter) UnmarshalJSON(data []byte) error {
	p := &CounterProduct{}
	if err := json.Unmarshal(data, p); err != nil {
		return err
	}
	fs.count = p.Count
	fs.value = p.Value
	return nil
}

func (fs *Counter) Add(v float64) {
	fs.Lock()
	defer fs.Unlock()
	fs.value += v
	fs.count++
}

func (fs *Counter) Produce(reset bool) Product {
	fs.Lock()
	defer fs.Unlock()
	ret := &CounterProduct{
		Count: int64(fs.count),
		Value: float64(fs.value),
	}
	if reset {
		fs.count = 0
		fs.value = 0
	}
	return ret
}

func (fs *Counter) String() string {
	return fs.Produce(false).String()
}

type CounterProduct struct {
	Count int64   `json:"count"`
	Value float64 `json:"value"`
}

func (cp *CounterProduct) String() string {
	b, _ := json.Marshal(cp)
	return string(b)
}
