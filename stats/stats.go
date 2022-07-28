package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"

	"ulambda/perf"
)

const STATS = true

type Tcounter uint64
type TCycles uint64

type UtilFn func() float64

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
	Nwatch      Tcounter
	Ncreate     Tcounter
	Nread       Tcounter
	Nwrite      Tcounter
	Nremove     Tcounter
	Nremovefile Tcounter
	Nstat       Tcounter
	Nwstat      Tcounter
	Nrenameat   Tcounter
	Nget        Tcounter
	Nset        Tcounter
	Nput        Tcounter

	Paths map[string]int

	Load       perf.Tload
	Util       float64
	CustomUtil float64
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
	case np.TTwatch:
		si.Nwatch.Inc()
	case np.TTrenameat:
		si.Nrenameat.Inc()
	case np.TTgetfile:
		si.Nget.Inc()
	case np.TTsetfile:
		si.Nset.Inc()
	case np.TTputfile:
		si.Nput.Inc()
	default:
	}
}

type Stats struct {
	fs.Inode
	mu            sync.Mutex // protects some fields of StatInfo
	sti           *StatInfo
	pid           int
	hz            int
	cores         map[string]bool
	monitoringCPU bool
	done          uint32
}

func MkStatsDev(parent fs.Dir) *Stats {
	st := &Stats{}
	st.Inode = inode.MakeInode(nil, np.DMDEVICE, parent)
	st.sti = MkStatInfo()
	st.pid = os.Getpid()
	return st
}

func (st *Stats) GetUtil() (float64, float64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.sti.Util, 0.0
}

func (st *Stats) GetLoad() perf.Tload {
	st.mu.Lock()
	defer st.mu.Unlock()

	load := perf.Tload{}
	load[0] = st.sti.Load[0]
	load[1] = st.sti.Load[1]
	load[2] = st.sti.Load[2]
	return load
}

func (st *Stats) StatInfo() *StatInfo {
	return st.sti
}

func (st *Stats) MonitorCPUUtil(ufn UtilFn) {
	st.hz = perf.Hz()
	// Don't duplicate work
	if !st.monitoringCPU {
		st.monitoringCPU = true
		go st.monitorCPUUtil(ufn)
	}
}

const (
	EXP_0 = 0.9048 // 1/exp(100ms/1000ms)
	EXP_1 = 0.9512 // 1/exp(100ms/2000ms)
	EXP_2 = 0.9801 // 1/exp(100ms/5000ms)
	MS    = 100    // 100 ms
	SEC   = 1000   // 1s
)

// Caller holds lock
func (st *Stats) loadCPUUtilL(idle, total uint64, customUtil float64) {
	util := 100.0 * (1.0 - float64(idle)/float64(total))

	st.sti.Load[0] *= EXP_0
	st.sti.Load[0] += (1 - EXP_0) * util
	st.sti.Load[1] *= EXP_1
	st.sti.Load[1] += (1 - EXP_1) * util
	st.sti.Load[2] *= EXP_2
	st.sti.Load[2] += (1 - EXP_2) * util

	st.sti.Util = util
	st.sti.CustomUtil = customUtil
}

// Update the set of cores we scan to determine CPU utilization.
func (st *Stats) UpdateCores() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.cores = perf.GetActiveCores()
}

func (st *Stats) monitorCPUUtil(ufn UtilFn) {
	total0 := uint64(0)
	total1 := uint64(0)
	idle0 := uint64(0)
	idle1 := uint64(0)

	st.UpdateCores()

	for atomic.LoadUint32(&st.done) != 1 {
		time.Sleep(time.Duration(MS) * time.Millisecond)

		// Can't call into ufn while the st lock is held, in order to ensure lock
		// ordering and avoid deadlock.
		var customUtil float64
		if ufn != nil {
			customUtil = ufn()
		}
		// Lock in case the set of cores we're monitoring changes.
		st.mu.Lock()
		idle1, total1 = perf.GetCPUSample(st.cores)
		st.loadCPUUtilL(idle1-idle0, total1-total0, customUtil)
		st.mu.Unlock()

		total0 = total1
		idle0 = idle1
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

func (st *Stats) IncPath(path np.Path) {
	st.mu.Lock()
	defer st.mu.Unlock()
	p := path.String()
	if _, ok := st.sti.Paths[p]; !ok {
		st.sti.Paths[p] = 0
	}
	st.sti.Paths[p] += 1
}

func (st *Stats) IncPathString(p string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.sti.Paths[p]; !ok {
		st.sti.Paths[p] = 0
	}
	st.sti.Paths[p] += 1
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
		db.DFatalf("stats: json failed %v\n", err)
	}
	return data
}

func (si *StatInfo) String() string {
	return fmt.Sprintf("&{ Nwalk:%v Nclunk:%v Nopen:%v Nwatch:%v Ncreate:%v Nflush:%v Nread:%v Nwrite:%v Nremove:%v Nstat:%v Nwstat:%v Nrenameat:%v Nget:%v Nset:%v Paths:%v Load:%v Util:%v }", si.Nwalk, si.Nclunk, si.Nopen, si.Nwatch, si.Ncreate, si.Nflush, si.Nread, si.Nwrite, si.Nremove, si.Nstat, si.Nwstat, si.Nrenameat, si.Nget, si.Nset, si.Paths, si.Load, si.Util)
}

func (st *Stats) Snapshot(fn fs.SnapshotF) []byte {
	return st.snapshot()
}
