package stats

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/util/perf"
	"sigmaos/util/spstats"
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
	SpSt spstats.SpStats
	Qlen spstats.Tcounter
	StatsCommon
}

func NewStats() *Stats {
	sti := &Stats{}
	return sti
}

func (si *Stats) Inc(fct sessp.Tfcall, ql int64) {
	st := &si.SpSt
	st.Inc(fct, ql)
	spstats.Inc(&si.Qlen, ql)
}

type StatInode struct {
	fs.Inode
	mu       sync.Mutex // protects some fields of StatInfo
	st       *Stats
	pathCnts bool
}

func NewStatsDev(ia *inode.InodeAlloc) *StatInode {
	sti := &StatInode{
		Inode:    ia.NewInode(nil, sp.DMDEVICE, sp.NoLeaseId),
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

func (sti *StatInode) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
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
	b := st.stats()
	db.DPrintf(db.STAT, "Read statinfo %v off %d %d sz %d", st, off, n, len(b))
	if st == nil {
		return nil, nil
	}
	if off > 0 {
		return nil, nil
	}
	// return no more data than asked for and that is available
	if sp.Tsize(len(b)) < n {
		n = sp.Tsize(len(b))
	}
	return b[:n], nil
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

type SrvStatsSnapshot struct {
	*spstats.TcounterSnapshot
	StatsCommon
}

func NewSrvStatsSnapshot() *SrvStatsSnapshot {
	return &SrvStatsSnapshot{TcounterSnapshot: spstats.NewTcounterSnapshot()}
}

func (sst *SrvStatsSnapshot) String() string {
	s := sst.TcounterSnapshot.String()
	c := &sst.StatsCommon
	s += c.String()
	return s
}

func (sti *StatInode) StatsSnapshot() *SrvStatsSnapshot {
	sp := &sti.st.SpSt
	spro := sp.StatsSnapshot()
	sti.mu.Lock()
	defer sti.mu.Unlock()
	stro := NewSrvStatsSnapshot()
	stro.TcounterSnapshot = spro
	stro.StatsCommon.AvgQlen = float64(sti.st.Qlen.Load()) / float64(sti.st.SpSt.Ntotal.Load())
	stro.Paths = sti.st.Paths
	stro.Util = sti.st.Util
	stro.Load = sti.st.Load
	stro.CustomUtil = sti.st.CustomUtil
	stro.CustomLoad = sti.st.CustomLoad
	return stro
}

func (sti *StatInode) stats() []byte {
	st := sti.StatsSnapshot()
	db.DPrintf(db.STAT, "stat %v\n", st)
	data, err := json.Marshal(st)
	if err != nil {
		db.DFatalf("stats: json marshaling failed %v", err)
	}
	return data
}
