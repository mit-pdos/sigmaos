package stats

import (
	"encoding/json"
	"log"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"ulambda/fslib"
	np "ulambda/ninep"

	"ulambda/perf"
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
type StatInfo struct {
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

	Paths map[string]int

	Util float64
}

func MkStatInfo() *StatInfo {
	sti := &StatInfo{}
	sti.Paths = make(map[string]int)
	return sti
}

type Stats struct {
	mu   sync.Mutex // protects some fields of StatInfo
	sti  *StatInfo
	pid  string
	hz   int
	done uint32
	fsl  *fslib.FsLib
}

func MkStats() *Stats {
	st := &Stats{}
	st.sti = MkStatInfo()
	return st
}

func (st *Stats) StatInfo() *StatInfo {
	return st.sti
}

func (st *Stats) MakeElastic(fsl *fslib.FsLib, pid string) {
	st.pid = pid
	st.fsl = fsl
	st.hz = perf.Hz()
	go st.monitorPID()
}

func (st *Stats) spawnMonitor() string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/monitor"
	a.Args = []string{}
	a.PairDep = nil
	a.ExitDep = nil
	st.fsl.Spawn(&a)
	return a.Pid
}

func (st *Stats) monitor() {
	pid := st.spawnMonitor()
	ok, err := st.fsl.Wait(pid)
	if string(ok) != "OK" || err != nil {
		log.Printf("monitor: ok %v err %v\n", string(ok), err)
	}
}

func (st *Stats) monitorPID() {
	ms := 1000
	j := 1000 / st.hz
	ncpu := runtime.GOMAXPROCS(0)
	var total0 uint64
	var total1 uint64
	pid := os.Getpid()
	total0 = perf.GetPIDSample(pid)
	first := true
	for atomic.LoadUint32(&st.done) != 1 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		total1 = perf.GetPIDSample(pid)
		delta := total1 - total0
		util := 100.0 * float64(delta) / float64(ms/j)
		util = util / float64(ncpu)

		st.mu.Lock()
		st.sti.Util = util
		st.mu.Unlock()

		if first {
			first = false
			continue
		}
		// log.Printf("CPU delta: %v util %f ncpu %v\n", delta, util, ncpu)
		if util >= perf.MAXLOAD {
			st.monitor()
		}
		if util < perf.MINLOAD {
			st.monitor()
		}
		total0 = total1
	}
}

func (st *Stats) Done() {
	atomic.StoreUint32(&st.done, 1)
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
	if _, ok := st.sti.Paths[path]; !ok {
		st.sti.Paths[path] = 0
	}
	st.sti.Paths[path] += 1
}

type pair struct {
	path string
	cnt  int
}

func (st *Stats) SortPathL() []pair {
	var s []pair

	for k, v := range st.sti.Paths {
		s = append(s, pair{k, v})
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].cnt > s[j].cnt
	})
	return s
}

// Make a copy of st while concurrent Inc()s may happen
func (sti *StatInfo) acopy() *StatInfo {
	sticp := &StatInfo{}

	v := reflect.ValueOf(sti).Elem()
	v1 := reflect.ValueOf(sticp).Elem()
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
	return sticp
}

func (st *Stats) stats() []byte {
	stcp := st.sti.acopy()
	st.mu.Lock()
	defer st.mu.Unlock()
	stcp.Paths = st.sti.Paths
	stcp.Util = st.sti.Util

	data, err := json.Marshal(*stcp)
	if err != nil {
		log.Fatalf("stats: json failed %v\n", err)
	}
	return data
}
