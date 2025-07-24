package metric

import (
	"strconv"
	"sync"
)

var _ Value[float64] = (*Gauge)(nil)

type Gauge struct {
	sync.Mutex
	value      float64
	extensions []Extension
}

func (g *Gauge) Extend(adder Extension) {
	g.extensions = append(g.extensions, adder)
}

func (g *Gauge) String() string {
	return strconv.FormatFloat(g.Value(), 'g', -1, 64)
}

func (g *Gauge) Value() float64 {
	g.Lock()
	defer g.Unlock()
	return g.value
}

func (g *Gauge) Mark(v float64) {
	g.Lock()
	defer g.Unlock()
	g.value = v
	for _, ext := range g.extensions {
		ext.Add(v)
	}
}

func (g *Gauge) Reset() {
	g.Lock()
	defer g.Unlock()
	g.value = 0
}
