package npobjsrv

import (
	"encoding/json"
	"log"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	np "ulambda/ninep"
)

const STATS = true
const TIMING = false

type Tcounter uint64
type TCycles uint64

func (c *Tcounter) Inc() {
	if STATS {
		n := (*uint64)(unsafe.Pointer(c))
		atomic.AddUint64(n, 1)
	}
}

// XXX separate cache lines
type Stats struct {
	Nwalk     Tcounter
	Nclunk    Tcounter
	Nopen     Tcounter
	Nwatchv   Tcounter
	Ncreate   Tcounter
	Nflush    Tcounter
	Nread     Tcounter
	Nreadv    Tcounter
	Nwrite    Tcounter
	Nwritev   Tcounter
	Nremove   Tcounter
	Nstat     Tcounter
	Nwstat    Tcounter
	Nrenameat Tcounter

	mu    sync.Mutex
	Paths map[string]int
}

func MkStats() *Stats {
	st := &Stats{}
	st.Paths = make(map[string]int)
	return st
}

func (st *Stats) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	return 0, nil
}

func (st *Stats) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	if st == nil {
		return nil, nil
	}
	if off > 0 {
		return nil, nil
	}
	b := st.stats()
	return b, nil
}

func (st *Stats) Len() np.Tlength {
	if st == nil {
		return 0
	}
	b := st.stats()
	return np.Tlength(len(b))
}

func (st *Stats) Path(p []string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	path := np.Join(p)
	if _, ok := st.Paths[path]; !ok {
		st.Paths[path] = 0
	}
	st.Paths[path] += 1
}

type pair struct {
	path string
	cnt  int
}

func (st *Stats) SortPathL() []pair {
	var s []pair

	for k, v := range st.Paths {
		s = append(s, pair{k, v})
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].cnt > s[j].cnt
	})
	return s
}

// Make a copy of st while concurrent Inc()s may happen
func (st *Stats) acopy() *Stats {
	stcp := &Stats{}

	v := reflect.ValueOf(st).Elem()
	v1 := reflect.ValueOf(stcp).Elem()
	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i).Type().String()
		if strings.HasSuffix(t, "Tcounter") {
			p := v.Field(i).Addr().Interface().(*Tcounter)
			ptr := (*uint64)(unsafe.Pointer(p))
			n := atomic.LoadUint64(ptr)
			p1 := v1.Field(i).Addr().Interface().(*Tcounter)
			*p1 = Tcounter(n)
		}
	}

	return stcp
}

func (st *Stats) stats() []byte {
	stcp := st.acopy()
	st.mu.Lock()
	defer st.mu.Unlock()
	stcp.Paths = st.Paths

	data, err := json.Marshal(*stcp)
	if err != nil {
		log.Fatalf("stats: json failed %v\n", err)
	}
	return data
}
