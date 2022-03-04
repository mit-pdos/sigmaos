package stats

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"ulambda/fs"
	"ulambda/inode"
	"ulambda/linuxsched"
	np "ulambda/ninep"

	"ulambda/perf"
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
	Nversion    Tcounter
	Nauth       Tcounter
	Nattach     Tcounter
	Nflush      Tcounter
	Nwalk       Tcounter
	Nclunk      Tcounter
	Nopen       Tcounter
	Nwatchv     Tcounter
	Ncreate     Tcounter
	Nread       Tcounter
	Nreadv      Tcounter
	Nwrite      Tcounter
	Nwritev     Tcounter
	Nremove     Tcounter
	Nremovefile Tcounter
	Nstat       Tcounter
	Nwstat      Tcounter
	Nrenameat   Tcounter
	Nget        Tcounter
	Nset        Tcounter
	Nput        Tcounter
	Nmkfence    Tcounter
	Nregfence   Tcounter
	Nunfence    Tcounter

	Paths map[string]int

	Load Tload
	Util float64
}

func MkStatInfo() *StatInfo {
	sti := &StatInfo{}
	sti.Paths = make(map[string]int)
	return sti
}

func (si *StatInfo) Inc(fct np.Tfcall) {
	switch fct {
	case np.TTversion:
		si.Nversion.Inc()
	case np.TTauth:
		si.Nauth.Inc()
	case np.TTattach:
		si.Nattach.Inc()
	case np.TTflush:
		si.Nflush.Inc()
	case np.TTwalk:
		si.Nwalk.Inc()
	case np.TTopen:
		si.Nopen.Inc()
	case np.TTcreate:
		si.Ncreate.Inc()
	case np.TTread:
		si.Nread.Inc()
	case np.TTwrite:
		si.Nwrite.Inc()
	case np.TTclunk:
		si.Nclunk.Inc()
	case np.TTremove:
		si.Nremove.Inc()
	case np.TTremovefile:
		si.Nremovefile.Inc()
	case np.TTstat:
		si.Nstat.Inc()
	case np.TTwstat:
		si.Nwstat.Inc()
	case np.TTwatchv:
		si.Nwatchv.Inc()
	case np.TTrenameat:
		si.Nrenameat.Inc()
	case np.TTgetfile:
		si.Nget.Inc()
	case np.TTsetfile:
		si.Nset.Inc()
	case np.TTputfile:
		si.Nput.Inc()
	case np.TTmkfence:
		si.Nmkfence.Inc()
	case np.TTregfence:
		si.Nregfence.Inc()
	case np.TTunfence:
		si.Nunfence.Inc()
	default:
	}
}

type Stats struct {
	fs.FsObj
	mu            sync.Mutex // protects some fields of StatInfo
	sti           *StatInfo
	pid           string
	hz            int
	monitoringCPU bool
	done          uint32
}

func MkStats(parent fs.Dir) *Stats {
	st := &Stats{}
	st.FsObj = inode.MakeInode(nil, np.DMDEVICE, parent)
	st.sti = MkStatInfo()
	return st
}

func (st *Stats) StatInfo() *StatInfo {
	return st.sti
}

func (st *Stats) MonitorCPUUtil() {
	st.hz = perf.Hz()
	// Don't duplicate work
	if !st.monitoringCPU {
		st.monitoringCPU = true
		go st.monitorCPUUtil()
	}
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

func (st *Stats) loadCPUUtil(idle, total uint64) {
	st.mu.Lock()
	defer st.mu.Unlock()

	util := 100.0 * (1.0 - float64(idle)/float64(total))

	nthread := float64(runtime.NumGoroutine())
	// log.Printf("loadCPUUtil %v %v nthread %v\n", idle, total, nthread)

	st.sti.Load[0] *= EXP_0
	st.sti.Load[0] += (1 - EXP_0) * nthread
	st.sti.Load[1] *= EXP_1
	st.sti.Load[1] += (1 - EXP_1) * nthread
	st.sti.Load[2] *= EXP_2
	st.sti.Load[2] += (1 - EXP_2) * nthread

	st.sti.Util = util
}

func (st *Stats) monitorCPUUtil() {
	total0 := uint64(0)
	total1 := uint64(0)
	idle0 := uint64(0)
	idle1 := uint64(0)
	pid := os.Getpid()
	period1 := 10 // 1000/MS;

	cores := map[string]bool{}

	linuxsched.ScanTopology()
	// Get the cores we can run on
	m, err := linuxsched.SchedGetAffinity(pid)
	if err != nil {
		log.Fatalf("Error getting affinity mask: %v", err)
	}
	for i := uint(0); i < linuxsched.NCores; i++ {
		if m.Test(i) {
			cores["cpu"+strconv.Itoa(int(i))] = true
		}
	}

	for atomic.LoadUint32(&st.done) != 1 {
		for i := 0; i < period1; i++ {
			time.Sleep(time.Duration(MS) * time.Millisecond)
			idle1, total1 = perf.GetCPUSample(cores)
			st.loadCPUUtil(idle1-idle0, total1-total0)
			total0 = total1
			idle0 = idle1
		}
	}
}

func (st *Stats) Done() {
	atomic.StoreUint32(&st.done, 1)
}

func (st *Stats) Write(ctx fs.CtxI, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, nil
}

func (st *Stats) Read(ctx fs.CtxI, off np.Toffset, n np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if st == nil {
		return nil, nil
	}
	if off > 0 {
		return nil, nil
	}
	b := st.stats()
	return b, nil
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

func (si *StatInfo) String() string {
	return fmt.Sprintf("&{ Nwalk:%v Nclunk:%v Nopen:%v Nwatchv:%v Ncreate:%v Nflush:%v Nread:%v Nreadv:%v Nwrite:%v Nwritev:%v Nremove:%v Nstat:%v Nwstat:%v Nrenameat:%v Nget:%v Nset:%v Paths:%v Load:%v Util:%v }", si.Nwalk, si.Nclunk, si.Nopen, si.Nwatchv, si.Ncreate, si.Nflush, si.Nread, si.Nreadv, si.Nwrite, si.Nwritev, si.Nremove, si.Nstat, si.Nwstat, si.Nrenameat, si.Nget, si.Nset, si.Paths, si.Load, si.Util)
}

func (st *Stats) Snapshot(fn fs.SnapshotF) []byte {
	return st.snapshot()
}
