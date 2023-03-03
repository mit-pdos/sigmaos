package stats

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/path"
	"sigmaos/perf"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const STATS = true

type Tcounter int64
type TCycles uint64

func (c *Tcounter) Inc() {
	if STATS {
		n := (*int64)(unsafe.Pointer(c))
		atomic.AddInt64(n, 1)
	}
}

func (c *Tcounter) Dec() {
	if STATS {
		n := (*int64)(unsafe.Pointer(c))
		atomic.AddInt64(n, -1)
	}
}

func (c *Tcounter) Read() int64 {
	if STATS {
		n := (*int64)(unsafe.Pointer(c))
		return atomic.LoadInt64(n)
	}
	return 0
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

	Util       float64
	CustomUtil float64

	Load       perf.Tload
	CustomLoad perf.Tload
}

func MkStatInfo() *StatInfo {
	sti := &StatInfo{}
	sti.Paths = make(map[string]int)
	return sti
}

func (si *StatInfo) Inc(fct sessp.Tfcall) {
	switch fct {
	case sessp.TTversion:
		si.Nversion.Inc()
	case sessp.TTauth:
		si.Nauth.Inc()
	case sessp.TTattach:
		si.Nattach.Inc()
	case sessp.TTflush:
		si.Nflush.Inc()
	case sessp.TTwalk:
		si.Nwalk.Inc()
	case sessp.TTopen:
		si.Nopen.Inc()
	case sessp.TTcreate:
		si.Ncreate.Inc()
	case sessp.TTread, sessp.TTreadV:
		si.Nread.Inc()
	case sessp.TTwrite, sessp.TTwriteV:
		si.Nwrite.Inc()
	case sessp.TTclunk:
		si.Nclunk.Inc()
	case sessp.TTremove:
		si.Nremove.Inc()
	case sessp.TTremovefile:
		si.Nremovefile.Inc()
	case sessp.TTstat:
		si.Nstat.Inc()
	case sessp.TTwstat:
		si.Nwstat.Inc()
	case sessp.TTwatch:
		si.Nwatch.Inc()
	case sessp.TTrenameat:
		si.Nrenameat.Inc()
	case sessp.TTgetfile:
		si.Nget.Inc()
	case sessp.TTputfile:
		si.Nput.Inc()
	default:
	}
}

type Stats struct {
	fs.Inode
	mu       sync.Mutex // protects some fields of StatInfo
	sti      *StatInfo
	pathCnts bool
}

func MkStatsDev(parent fs.Dir) *Stats {
	st := &Stats{}
	st.Inode = inode.MakeInode(nil, sp.DMDEVICE, parent)
	st.sti = MkStatInfo()
	st.pathCnts = true
	return st
}

func (st *Stats) SetLoad(load perf.Tload, cload perf.Tload, u, cu float64) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.sti.Load = load
	st.sti.CustomLoad = cload
	st.sti.Util = u
	st.sti.CustomUtil = cu
}

func (st *Stats) StatInfo() *StatInfo {
	return st.sti
}

func (st *Stats) Write(ctx fs.CtxI, off sp.Toffset, data []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	return 0, nil
}

func (st *Stats) Read(ctx fs.CtxI, off sp.Toffset, n sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	if st == nil {
		return nil, nil
	}
	if off > 0 {
		return nil, nil
	}
	b := st.stats()
	return b, nil
}

func (st *Stats) DisablePathCnts() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.pathCnts = false
	st.sti.Paths = nil
}

func (st *Stats) IncPath(path path.Path) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if !st.pathCnts {
		return
	}
	p := path.String()
	if _, ok := st.sti.Paths[p]; !ok {
		st.sti.Paths[p] = 0
	}
	st.sti.Paths[p] += 1
}

func (st *Stats) IncPathString(p string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if !st.pathCnts {
		return
	}
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
	stcp.CustomUtil = st.sti.CustomUtil
	stcp.CustomLoad = st.sti.CustomLoad

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
