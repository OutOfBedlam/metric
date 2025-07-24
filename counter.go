package metric

import (
	"fmt"
	"sync/atomic"
)

var _ Value[int64] = (*Counter)(nil)

type Counter struct {
	value      atomic.Int64
	extensions []Extension
}

func (c *Counter) Extend(ext Extension) {
	c.extensions = append(c.extensions, ext)
}

func (c *Counter) String() string {
	return fmt.Sprintf("%d", c.Value())
}

func (c *Counter) Value() int64 {
	return c.value.Load()
}

func (c *Counter) Mark(v int64) {
	val := c.value.Add(v)
	for _, ext := range c.extensions {
		ext.Add(float64(val))
	}
}

func (c *Counter) Reset() {
	c.value.Store(0)
}
