package fsetcd

import (
	"encoding/json"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/sigmasrv/stats"
	"sigmaos/util/syncmap"
)

type pstats struct {
	paths *syncmap.SyncMap[string, *stats.Tcounter]
}

func newPstats() *pstats {
	return &pstats{paths: syncmap.NewSyncMap[string, *stats.Tcounter]()}
}

func (ps *pstats) Update(pn path.Tpathname, c stats.Tcounter) {
	c0, _ := ps.paths.AllocNew(pn.String(), func(string) *stats.Tcounter {
		n := stats.NewCounter(0)
		return &n
	})
	stats.Add(c0, c)
}

// For reading and marshaling
type PstatsSnapshot struct {
	Counters map[string]int64
}

func NewPstatsSnapshot() *PstatsSnapshot {
	return &PstatsSnapshot{Counters: make(map[string]int64)}
}

type PstatInode struct {
	fs.Inode
	pstats *pstats
}

func NewPstatsDev() *PstatInode {
	sti := &PstatInode{
		Inode:  inode.NewInode(nil, sp.DMDEVICE, sp.NoLeaseId),
		pstats: newPstats(),
	}
	return sti
}

func (sti *PstatInode) stats() []byte {
	ss := NewPstatsSnapshot()
	sti.pstats.paths.Iter(func(k string, c *stats.Tcounter) bool {
		ss.Counters[k] = c.Load()
		return true
	})
	data, err := json.Marshal(ss)
	if err != nil {
		db.DFatalf("stats: json marshaling failed %v", err)
	}
	return data
}

func (sti *PstatInode) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
	st, err := sti.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	b := sti.stats()
	st.SetLengthInt(len(b))
	return st, nil
}

func (sti *PstatInode) Write(ctx fs.CtxI, off sp.Toffset, data []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, nil
}

func (sti *PstatInode) Read(ctx fs.CtxI, off sp.Toffset, n sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	db.DPrintf(db.TEST, "Read statinfo %v\n", sti)
	if sti == nil {
		return nil, nil
	}
	if off > 0 {
		return nil, nil
	}
	b := sti.stats()
	return b, nil
}
