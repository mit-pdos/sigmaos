package npobjsrv

import (
	"reflect"
	"sort"
	"strconv"
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
	paths map[string]int
}

func MkStats() *Stats {
	st := &Stats{}
	st.paths = make(map[string]int)
	return st
}

func (st *Stats) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	return 0, nil
}

func (st *Stats) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	if st == nil {
		return nil, nil
	}
	b := []byte(st.String())
	return b, nil
}

func (st *Stats) Len() np.Tlength {
	if st == nil {
		return 0
	}
	b := []byte(st.String())
	return np.Tlength(len(b))
}

func (st *Stats) Path(p []string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	path := np.Join(p)
	if _, ok := st.paths[path]; !ok {
		st.paths[path] = 0
	}
	st.paths[path] += 1
}

type pair struct {
	path string
	cnt  int
}

func (st *Stats) SortPathL() []pair {
	var s []pair

	for k, v := range st.paths {
		s = append(s, pair{k, v})
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].cnt > s[j].cnt
	})
	return s
}

func (st *Stats) String() string {
	st.mu.Lock()
	defer st.mu.Unlock()

	v := reflect.ValueOf(st).Elem()
	s := ""
	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i).Type().String()
		if strings.HasSuffix(t, "Tcounter") {
			p := v.Field(i).Addr().Interface().(*Tcounter)
			ptr := (*uint64)(unsafe.Pointer(p))
			n := atomic.LoadUint64(ptr)
			s += "#" + v.Type().Field(i).Name + ": " + strconv.FormatInt(int64(n), 10) + "\n"
		}
	}
	s = s + "\nTop paths:\n"
	ss := st.SortPathL()
	for _, p := range ss {
		s += p.path + ":" + strconv.Itoa(p.cnt) + "\n"
	}
	return s
}
