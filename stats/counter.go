package stats

import (
	"sync/atomic"
)

const STATS = true

type Tcounter = atomic.Int64

func NewCounter(n int64) Tcounter {
	c := Tcounter{}
	c.Store(n)
	return c
}

func Inc(c *Tcounter, v int64) {
	if STATS {
		c.Add(v)
	}
}

func Add(c *Tcounter, v Tcounter) {
	if STATS {
		c.Add(v.Load())
	}
}

func Dec(c *Tcounter) {
	if STATS {
		c.Add(-1)
	}
}

func Max(max *Tcounter, v int64) {
	if STATS {
		for {
			old := max.Load()
			if old == 0 || v > old {
				if ok := max.CompareAndSwap(old, v); ok {
					return
				}
				// retry
			} else {
				return
			}
		}
	}
}
