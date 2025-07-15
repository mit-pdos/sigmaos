package spstats

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/mitchellh/mapstructure"

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

type TcounterSnapshot struct {
	Counters map[string]int64
}

func NewTcounterSnapshot() *TcounterSnapshot {
	return &TcounterSnapshot{Counters: make(map[string]int64)}
}

func (cnts *TcounterSnapshot) FillCounters(st any) {
	v := reflect.ValueOf(st).Elem()
	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i).Type().String()
		n := v.Type().Field(i).Name
		if strings.HasSuffix(t, "atomic.Int64") {
			p := v.Field(i).Addr().Interface().(*atomic.Int64)
			cnts.Counters[n] = p.Load()
		}
	}
}

func (cnts *TcounterSnapshot) String() string {
	ks := make([]string, 0, len(cnts.Counters))
	for k := range cnts.Counters {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	s := "["
	for _, k := range ks {
		s += fmt.Sprintf("{%s: %d}", k, cnts.Counters[k])
	}
	s += "] "
	return s
}

func UnmarshalTcounterSnapshot(d any) (*TcounterSnapshot, error) {
	st := NewTcounterSnapshot()
	err := mapstructure.Decode(d, &st)
	return st, err
}
