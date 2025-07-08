package spstats

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
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

func FillCounters(st any, counters map[string]int64) {
	v := reflect.ValueOf(st).Elem()
	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i).Type().String()
		n := v.Type().Field(i).Name
		if strings.HasSuffix(t, "atomic.Int64") {
			p := v.Field(i).Addr().Interface().(*atomic.Int64)
			counters[n] = p.Load()
		}
	}
}

func StringCounters(counters map[string]int64) string {
	ks := make([]string, 0, len(counters))
	for k := range counters {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	s := "["
	for _, k := range ks {
		s += fmt.Sprintf("{%s: %d}", k, counters[k])
	}
	s += "] "
	return s
}
