package stats

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfs/inode"
	"sigmaos/path"
	"sigmaos/perf"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type StatsCommon struct {
	Paths map[string]int

	AvgQlen float64

	Util       float64
	CustomUtil float64

	Load       perf.Tload
	CustomLoad perf.Tload
}

func (st *StatsCommon) String() string {
	return fmt.Sprintf("&{AvgQlen: %.3f Paths:%v Load:%v Util:%v }", st.AvgQlen, st.Paths, st.Load, st.Util)
}

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

	Qlen Tcounter

	StatsCommon
}

// For reading and marshaling
type StatsSnapshot struct {
	Counters map[string]int64
	StatsCommon
}

func newStatsSnapshot() *StatsSnapshot {
	st := &StatsSnapshot{}
	st.Counters = make(map[string]int64)
	return st
}

func (st *StatsSnapshot) String() string {
	ks := make([]string, 0, len(st.Counters))
	for k := range st.Counters {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	s := "["
	for _, k := range ks {
		s += fmt.Sprintf("{%s: %d}", k, st.Counters[k])
	}
	s += "] "
	c := &st.StatsCommon
	s += c.String()
	return s
}

func NewStats() *Stats {
	sti := &Stats{}
	return sti
}

func (si *Stats) Inc(fct sessp.Tfcall, ql int64) {
	switch fct {
	case sessp.TTversion:
		Inc(&si.Nversion, 1)
	case sessp.TTauth:
		Inc(&si.Nauth, 1)
	case sessp.TTattach:
		Inc(&si.Nattach, 1)
	case sessp.TTdetach:
		Inc(&si.Ndetach, 1)
	case sessp.TTflush:
		Inc(&si.Nflush, 1)
	case sessp.TTwalk:
		Inc(&si.Nwalk, 1)
	case sessp.TTopen:
		Inc(&si.Nopen, 1)
	case sessp.TTcreate:
		Inc(&si.Ncreate, 1)
	case sessp.TTread, sessp.TTreadF:
		Inc(&si.Nread, 1)
	case sessp.TTwrite, sessp.TTwriteF:
		Inc(&si.Nwrite, 1)
	case sessp.TTclunk:
		Inc(&si.Nclunk, 1)
	case sessp.TTremove:
		Inc(&si.Nremove, 1)
	case sessp.TTremovefile:
		Inc(&si.Nremovefile, 1)
	case sessp.TTstat:
		Inc(&si.Nstat, 1)
	case sessp.TTwstat:
		Inc(&si.Nwstat, 1)
	case sessp.TTwatch:
		Inc(&si.Nwatch, 1)
	case sessp.TTrenameat:
		Inc(&si.Nrenameat, 1)
	case sessp.TTgetfile:
		Inc(&si.Nget, 1)
	case sessp.TTputfile:
		Inc(&si.Nput, 1)
	case sessp.TTwriteread:
		Inc(&si.Nrpc, 1)
	default:
		db.DPrintf(db.ALWAYS, "StatInfo: missing counter for %v\n", fct)
	}
	Inc(&si.Ntotal, 1)
	Inc(&si.Qlen, ql)
}

type StatInode struct {
	fs.Inode
	mu       sync.Mutex // protects some fields of StatInfo
	st       *Stats
	pathCnts bool
}

func NewStatsDev(parent fs.Dir) *StatInode {
	sti := &StatInode{
		Inode:    inode.NewInode(nil, sp.DMDEVICE, sp.NoLeaseId, parent),
		st:       NewStats(),
		pathCnts: false,
	}
	return sti
}

func (sti *StatInode) SetLoad(load perf.Tload, cload perf.Tload, u, cu float64) {
	sti.mu.Lock()
	defer sti.mu.Unlock()

	sti.st.Load = load
	sti.st.CustomLoad = cload
	sti.st.Util = u
	sti.st.CustomUtil = cu
}

func (sti *StatInode) Stats() *Stats {
	return sti.st
}

func (sti *StatInode) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := sti.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	b := sti.stats()
	st.SetLengthInt(len(b))
	return st, nil
}

func (st *StatInode) Write(ctx fs.CtxI, off sp.Toffset, data []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, nil
}

func (st *StatInode) Read(ctx fs.CtxI, off sp.Toffset, n sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	db.DPrintf(db.TEST, "Read statinfo %v\n", st)
	if st == nil {
		return nil, nil
	}
	if off > 0 {
		return nil, nil
	}
	b := st.stats()
	return b, nil
}

func (sti *StatInode) EnablePathCnts() {
	sti.mu.Lock()
	defer sti.mu.Unlock()

	sti.pathCnts = true
	sti.st.Paths = make(map[string]int)
}

func (sti *StatInode) IncPath(path path.Tpathname) {
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

func (sti *StatInode) IncPathString(p string) {
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

// Make a StatsSnapshot from st while concurrent Inc()s may happen
func (st *Stats) statsSnapshot() *StatsSnapshot {
	stro := newStatsSnapshot()

	v := reflect.ValueOf(st).Elem()
	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i).Type().String()
		n := v.Type().Field(i).Name
		if strings.HasSuffix(t, "atomic.Int64") {
			p := v.Field(i).Addr().Interface().(*atomic.Int64)
			stro.Counters[n] = p.Load()
		}
	}
	return stro
}

func (sti *StatInode) StatsSnapshot() *StatsSnapshot {
	stro := sti.st.statsSnapshot()
	sti.mu.Lock()
	defer sti.mu.Unlock()
	stro.AvgQlen = float64(sti.st.Qlen.Load()) / float64(sti.st.Ntotal.Load())
	stro.Paths = sti.st.Paths
	stro.Util = sti.st.Util
	stro.Load = sti.st.Load
	stro.CustomUtil = sti.st.CustomUtil
	stro.CustomLoad = sti.st.CustomLoad
	return stro
}

func (sti *StatInode) stats() []byte {
	st := sti.StatsSnapshot()
	db.DPrintf(db.TEST, "stat %v\n", st)
	data, err := json.Marshal(st)
	if err != nil {
		db.DFatalf("stats: json marshaling failed %v", err)
	}
	return data
}
