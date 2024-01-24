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
type Stats struct {
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

func NewStats() *Stats {
	sti := &Stats{}
	return sti
}

func (si *Stats) Inc(fct sessp.Tfcall, ql int64) {
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
	case sessp.TTread, sessp.TTreadF:
		si.Nread.Inc(1)
	case sessp.TTwrite, sessp.TTwriteF:
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

type StatInfo struct {
	fs.Inode
	mu       sync.Mutex // protects some fields of StatInfo
	st       *Stats
	pathCnts bool
}

func NewStatsDev(parent fs.Dir) *StatInfo {
	sti := &StatInfo{}
	sti.Inode = inode.NewInode(nil, sp.DMDEVICE, parent)
	sti.st = NewStats()
	sti.pathCnts = false
	return sti
}

func (sti *StatInfo) SetLoad(load perf.Tload, cload perf.Tload, u, cu float64) {
	sti.mu.Lock()
	defer sti.mu.Unlock()

	sti.st.Load = load
	sti.st.CustomLoad = cload
	sti.st.Util = u
	sti.st.CustomUtil = cu
}

func (sti *StatInfo) Stats() *Stats {
	return sti.st
}

func (st *StatInfo) Write(ctx fs.CtxI, off sp.Toffset, data []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, nil
}

func (st *StatInfo) Read(ctx fs.CtxI, off sp.Toffset, n sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	if st == nil {
		return nil, nil
	}
	if off > 0 {
		return nil, nil
	}
	b := st.stats()
	return b, nil
}

func (sti *StatInfo) EnablePathCnts() {
	sti.mu.Lock()
	defer sti.mu.Unlock()

	sti.pathCnts = true
	sti.st.Paths = make(map[string]int)
}

func (sti *StatInfo) IncPath(path path.Path) {
	sti.mu.Lock()
	defer sti.mu.Unlock()

	if !sti.pathCnts {
		return
	}
	p := path.String()
	if _, ok := sti.st.Paths[p]; !ok {
		sti.st.Paths[p] = 0
	}
	sti.st.Paths[p] += 1
}

func (sti *StatInfo) IncPathString(p string) {
	sti.mu.Lock()
	defer sti.mu.Unlock()

	if !sti.pathCnts {
		return
	}
	if _, ok := sti.st.Paths[p]; !ok {
		sti.st.Paths[p] = 0
	}
	sti.st.Paths[p] += 1
}

type pair struct {
	path string
	cnt  int
}

func (st *Stats) SortPath() []pair {
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
func (st *Stats) acopy() Stats {
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
	return *stcp
}

func (sti *StatInfo) StatsCopy() Stats {
	stcp := sti.st.acopy()
	sti.mu.Lock()
	defer sti.mu.Unlock()
	stcp.AvgQlen = float64(sti.st.Qlen) / float64(sti.st.Ntotal)
	stcp.Paths = sti.st.Paths
	stcp.Util = sti.st.Util
	stcp.Load = sti.st.Load
	stcp.CustomUtil = sti.st.CustomUtil
	stcp.CustomLoad = sti.st.CustomLoad
	return stcp
}

func (sti *StatInfo) stats() []byte {
	st := sti.StatsCopy()
	data, err := json.Marshal(st)
	if err != nil {
		db.DFatalf("stats: json marshaling failed %v", err)
	}
	return data
}

func (st *Stats) String() string {
	return fmt.Sprintf("&{ Ntotal:%v Nattach:%v Ndetach:%v Nwalk:%v Nclunk:%v Nopen:%v Nwatch:%v Ncreate:%v Nflush:%v Nread:%v Nwrite:%v Nremove:%v Nstat:%v Nwstat:%v Nrenameat:%v Nget:%v Nput:%v Nrpc: %v Qlen: %v AvgQlen: %.3f Paths:%v Load:%v Util:%v }", st.Ntotal, st.Nattach, st.Ndetach, st.Nwalk, st.Nclunk, st.Nopen, st.Nwatch, st.Ncreate, st.Nflush, st.Nread, st.Nwrite, st.Nremove, st.Nstat, st.Nwstat, st.Nrenameat, st.Nget, st.Nput, st.Nrpc, st.Qlen, st.AvgQlen, st.Paths, st.Load, st.Util)
}
