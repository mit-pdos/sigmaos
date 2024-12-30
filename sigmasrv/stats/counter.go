package stats

import (
	"sync/atomic"

	sp "sigmaos/sigmap"
)

type Tcounter = atomic.Int64

func NewCounter(n int64) Tcounter {
	c := Tcounter{}
	c.Store(n)
	return c
}

func Inc(c *Tcounter, v int64) {
	if sp.Conf.Util.STATS {
		c.Add(v)
	}
}

func Add(c *Tcounter, v Tcounter) {
	if sp.Conf.Util.STATS {
		c.Add(v.Load())
	}
}

func Dec(c *Tcounter) {
	if sp.Conf.Util.STATS {
		c.Add(-1)
	}
}

func Max(max *Tcounter, v int64) {
	if sp.Conf.Util.STATS {
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
