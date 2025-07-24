package metric

import (
	"sync"
	"time"
)

var _ Value[time.Duration] = (*Timer)(nil)

type TimerExtendPoint int

const (
	TimerExtendValue TimerExtendPoint = iota
	TimerExtendCount
	TimerExtendTotal
	TimerExtendMin
	TimerExtendMax
	TimerExtendAvg
)

type Timer struct {
	sync.Mutex
	count         int64
	totalDuration time.Duration
	minDuration   time.Duration
	maxDuration   time.Duration
	exts          map[TimerExtendPoint]Extension
}

func (t *Timer) Extend(point TimerExtendPoint, ext Extension) {
	t.Lock()
	defer t.Unlock()
	if t.exts == nil {
		t.exts = make(map[TimerExtendPoint]Extension)
	}
	if ext == nil {
		delete(t.exts, point)
		return
	}
	t.exts[point] = ext
}

func (t *Timer) String() string {
	return time.Duration(t.Value()).String()
}

func (t *Timer) Value() time.Duration {
	t.Lock()
	defer t.Unlock()
	if t.count == 0 {
		return 0
	}
	return time.Duration(int64(t.totalDuration) / t.count)
}

type TimerSnapshot struct {
	Count         int64         `json:"count"`
	TotalDuration time.Duration `json:"total"`
	MinDuration   time.Duration `json:"min"`
	MaxDuration   time.Duration `json:"max"`
}

func (t *Timer) Snapshot() TimerSnapshot {
	t.Lock()
	defer t.Unlock()
	return TimerSnapshot{
		Count:         t.count,
		TotalDuration: t.totalDuration,
		MinDuration:   t.minDuration,
		MaxDuration:   t.maxDuration,
	}
}

func (t *Timer) Mark(d time.Duration) {
	t.Lock()
	defer t.Unlock()
	t.count++
	t.totalDuration += d
	if t.minDuration == 0 || d < t.minDuration {
		t.minDuration = d
	}
	if d > t.maxDuration {
		t.maxDuration = d
	}
	for point, ext := range t.exts {
		switch point {
		case TimerExtendValue:
			ext.Add(float64(d.Nanoseconds()))
		case TimerExtendCount:
			ext.Add(float64(t.count))
		case TimerExtendTotal:
			ext.Add(float64(t.totalDuration))
		case TimerExtendMin:
			ext.Add(float64(t.minDuration))
		case TimerExtendMax:
			ext.Add(float64(t.maxDuration))
		case TimerExtendAvg:
			if t.count > 0 {
				ext.Add(float64(t.totalDuration) / float64(t.count))
			}
		}
	}
}

func (t *Timer) Reset() {
	t.Lock()
	defer t.Unlock()
	t.count = 0
	t.totalDuration = 0
	t.minDuration = 0
	t.maxDuration = 0
}
