package stats

import (
	"encoding/json"
	"fmt"
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

	"ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"

	"ulambda/perf"
)

const (
	MAXLOAD float64 = 85.0
	MINLOAD float64 = 40.0
)

const STATS = true

// const TIMING = false

type Tcounter uint64
type TCycles uint64
type Tload [3]float64

func (t Tload) String() string {
	return fmt.Sprintf("[%.1f %.1f %.1f]", t[0], t[1], t[2])
}

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
	Nget      Tcounter
	Nset      Tcounter

	Paths map[string]int

	Load Tload
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
	*proc.ProcCtl
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
	st.ProcCtl = proc.MakeProcCtl(fsl)
	st.hz = perf.Hz()
	runtime.GOMAXPROCS(2) // XXX for KV
	go st.monitorPID()
}

func (st *Stats) spawnMonitor() string {
	p := proc.Proc{}
	p.Pid = fslib.GenPid()
	p.Program = "bin/user/monitor"
	p.Args = []string{}
	p.Type = proc.T_LC
	st.Spawn(&p)
	return p.Pid
}

func (st *Stats) monitor() {
	t0 := time.Now().UnixNano()
	pid := st.spawnMonitor()
	err := st.WaitExit(pid)
	if err != nil {
		log.Printf("monitor: err %v\n", err)
	}
	t1 := time.Now().UnixNano()
	log.Printf("mon: %v\n", t1-t0)
}

const (
	EXP_0 = 0.9048 // 1/exp(100ms/1000ms)
	EXP_1 = 0.9512 // 1/exp(100ms/2000ms)
	EXP_2 = 0.9801 // 1/exp(100ms/5000ms)
	MS    = 100    // 100 ms
	SEC   = 1000   // 1s
)

// XXX nthread includes 2 threads per conn
func (st *Stats) load(ticks uint64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	j := SEC / st.hz
	ncpu := runtime.GOMAXPROCS(0)

	util := 100.0 * float64(ticks) / float64(MS/j)
	util = util / float64(ncpu)

	nthread := float64(runtime.NumGoroutine())

	st.sti.Load[0] *= EXP_0
	st.sti.Load[0] += (1 - EXP_0) * nthread
	st.sti.Load[1] *= EXP_1
	st.sti.Load[1] += (1 - EXP_1) * nthread
	st.sti.Load[2] *= EXP_2
	st.sti.Load[2] += (1 - EXP_2) * nthread

	st.sti.Util = util
}

// XXX too simplistic?
func isDelay(load Tload) bool {
	return load[0] > load[1] && load[1] > load[2]
}

func noDelay(load Tload) bool {
	return load[0] < load[1] && load[1] < load[2]
}

func (st *Stats) doMonitor() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	log.Printf("%v: u %.1f l %v\n", debug.GetName(), st.sti.Util, st.sti.Load)
	if st.sti.Util >= MAXLOAD && isDelay(st.sti.Load) {
		return true
	}
	if st.sti.Util < MINLOAD && noDelay(st.sti.Load) {
		return true
	}
	return false
}

func (st *Stats) monitorPID() {
	total0 := uint64(0)
	total1 := uint64(0)
	pid := os.Getpid()
	total0 = perf.GetPIDSample(pid)
	period1 := 10 // 1000/MS;

	for atomic.LoadUint32(&st.done) != 1 {
		for i := 0; i < period1; i++ {
			time.Sleep(time.Duration(MS) * time.Millisecond)
			total1 = perf.GetPIDSample(pid)
			st.load(total1 - total0)
			total0 = total1
		}
		if st.doMonitor() {
			st.monitor()
		}
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

func (sti *StatInfo) SortPath() []pair {
	var s []pair

	for k, v := range sti.Paths {
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
	stcp.Load = st.sti.Load

	data, err := json.Marshal(*stcp)
	if err != nil {
		log.Fatalf("stats: json failed %v\n", err)
	}
	return data
}
