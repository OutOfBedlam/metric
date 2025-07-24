package metric

import (
	"fmt"
	"math"
	"sync"
)

type Bin struct {
	value float64
	count float64
}

type Histogram struct {
	sync.Mutex
	maxBins int
	bins    []Bin
	total   float64
}

func NewHistogram(maxBins int) *Histogram {
	return &Histogram{
		maxBins: maxBins,
	}
}

func (h *Histogram) String() string {
	h.Lock()
	defer h.Unlock()
	v := h.Quantiles(0.5, 0.90, 0.99)
	return fmt.Sprintf(`{"p50":%f,"p90":%f, "p99":%f}`, v[0], v[1], v[2])
}

func (h *Histogram) Reset() {
	h.Lock()
	defer h.Unlock()
	h.bins = nil
	h.total = 0
}

func (h *Histogram) Add(value float64) {
	h.Lock()
	defer func() {
		h.trim()
		h.Unlock()
	}()

	h.total++
	newBin := Bin{value: value, count: 1}
	for i := range h.bins {
		if h.bins[i].value > value {
			h.bins = append(h.bins[:i], append([]Bin{newBin}, h.bins[i:]...)...)
			return
		}
	}
	h.bins = append(h.bins, newBin)
}

func (h *Histogram) trim() {
	if h.maxBins <= 0 {
		h.maxBins = 100
	}
	for len(h.bins) > h.maxBins {
		d := float64(0)
		i := 0
		for j := 1; j < len(h.bins); j++ {
			if dv := h.bins[j].value - h.bins[j-1].value; dv < d || j == 1 {
				d = dv
				i = j
			}
		}
		count := h.bins[i].count + h.bins[i-1].count
		merged := Bin{
			value: (h.bins[i].value*h.bins[i].count + h.bins[i-1].value*h.bins[i-1].count) / count,
			count: count,
		}
		h.bins = append(h.bins[:i-1], h.bins[i:]...)
		h.bins[i-1] = merged
	}
}

func (h *Histogram) bin(q float64) Bin {
	count := q * float64(h.total)
	for i := range h.bins {
		count -= h.bins[i].count
		if count <= 0 {
			return h.bins[i]
		}
	}
	return Bin{}
}

func (h *Histogram) Quantile(q float64) float64 {
	h.Lock()
	defer h.Unlock()
	return h.bin(q).value
}

func (h *Histogram) Quantiles(qs ...float64) []float64 {
	h.Lock()
	defer h.Unlock()
	ret := make([]float64, len(qs))
	counts := make([]float64, len(qs))
	for i, q := range qs {
		counts[i] = q * float64(h.total)
	}
	found := 0
	for i := range h.bins {
		for idx := range counts {
			if counts[idx] == counts[idx] {
				counts[idx] -= h.bins[i].count
				if counts[idx] <= 0 {
					ret[idx] = h.bins[i].value
					counts[idx] = math.NaN() // Mark as found
					found++
				}
			}
		}
		if found == len(qs) {
			break
		}
	}
	return ret
}
