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

func (c *Tcounter) Inc(v int64) {
	if STATS {
		n := (*int64)(unsafe.Pointer(c))
		atomic.AddInt64(n, v)
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
	Ntotal      Tcounter
	Nversion    Tcounter
	Nauth       Tcounter
	Nattach     Tcounter
	Ndetach     Tcounter
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
	Nput        Tcounter
	Nrpc        Tcounter

	Paths map[string]int

	Qlen    Tcounter
	AvgQlen float64

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

func (si *StatInfo) Inc(fct sessp.Tfcall, ql int64) {
	switch fct {
	case sessp.TTversion:
		si.Nversion.Inc(1)
	case sessp.TTauth:
		si.Nauth.Inc(1)
	case sessp.TTattach:
		si.Nattach.Inc(1)
	case sessp.TTdetach:
		si.Ndetach.Inc(1)
	case sessp.TTflush:
		si.Nflush.Inc(1)
	case sessp.TTwalk:
		si.Nwalk.Inc(1)
	case sessp.TTopen:
		si.Nopen.Inc(1)
	case sessp.TTcreate:
		si.Ncreate.Inc(1)
	case sessp.TTread, sessp.TTreadV:
		si.Nread.Inc(1)
	case sessp.TTwrite, sessp.TTwriteV:
		si.Nwrite.Inc(1)
	case sessp.TTclunk:
		si.Nclunk.Inc(1)
	case sessp.TTremove:
		si.Nremove.Inc(1)
	case sessp.TTremovefile:
		si.Nremovefile.Inc(1)
	case sessp.TTstat:
		si.Nstat.Inc(1)
	case sessp.TTwstat:
		si.Nwstat.Inc(1)
	case sessp.TTwatch:
		si.Nwatch.Inc(1)
	case sessp.TTrenameat:
		si.Nrenameat.Inc(1)
	case sessp.TTgetfile:
		si.Nget.Inc(1)
	case sessp.TTputfile:
		si.Nput.Inc(1)
	case sessp.TTwriteread:
		si.Nrpc.Inc(1)
	default:
		db.DPrintf(db.ALWAYS, "StatInfo: missing counter for %v\n", fct)
	}
	si.Ntotal.Inc(1)
	si.Qlen.Inc(ql)
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
	stcp.AvgQlen = float64(st.sti.Qlen) / float64(st.sti.Ntotal)
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
	return fmt.Sprintf("&{ Ntotal:%v Nattach:%v Ndetach:%v Nwalk:%v Nclunk:%v Nopen:%v Nwatch:%v Ncreate:%v Nflush:%v Nread:%v Nwrite:%v Nremove:%v Nstat:%v Nwstat:%v Nrenameat:%v Nget:%v Nput:%v Nrpc: %v Qlen: %v AvgQlen: %.3f Paths:%v Load:%v Util:%v }", si.Ntotal, si.Nattach, si.Ndetach, si.Nwalk, si.Nclunk, si.Nopen, si.Nwatch, si.Ncreate, si.Nflush, si.Nread, si.Nwrite, si.Nremove, si.Nstat, si.Nwstat, si.Nrenameat, si.Nget, si.Nput, si.Nrpc, si.Qlen, si.AvgQlen, si.Paths, si.Load, si.Util)
}

func (st *Stats) Snapshot(fn fs.SnapshotF) []byte {
	return st.snapshot()
}
