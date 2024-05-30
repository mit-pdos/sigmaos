package procfs

import (
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sortedmap"
)

type ProcDir struct {
	sync.Mutex
	*ProcInode
	procs ProcFs
	sd    *sortedmap.SortedMap[string, *sp.Tstat]
}

func NewProcDir(procs ProcFs) fs.FsObj {
	return &ProcDir{ProcInode: newProcInode(sp.DMDIR|0555, "pids"), procs: procs}
}

func (pd *ProcDir) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	ps := pd.procs.GetProcs()
	pd.sd = sortedmap.NewSortedMap[string, *sp.Tstat]()
	for _, p := range ps {
		n := string(p.GetPid())
		pi := newProcInode(0444, n)
		st, _ := pi.Stat(nil)
		pd.sd.Insert(n, st)
	}
	return nil, nil
}

func (pd *ProcDir) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	pd.sd = nil
	return nil
}

func (pd *ProcDir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, f sp.Tfence) (fs.FsObj, *serr.Err) {
	return nil, serr.NewErr(serr.TErrNotSupported, "Create")
}

func (pd *ProcDir) Remove(ctx fs.CtxI, name string, f sp.Tfence) *serr.Err {
	return serr.NewErr(serr.TErrNotSupported, "Remove")
}

func (pd *ProcDir) Rename(ctx fs.CtxI, from, to string, f sp.Tfence) *serr.Err {
	return serr.NewErr(serr.TErrNotSupported, "Rename")
}

func (pd *ProcDir) Renameat(ctx fs.CtxI, from string, dd fs.Dir, to string, f sp.Tfence) *serr.Err {
	return serr.NewErr(serr.TErrNotSupported, "Renameat")
}

func (pd *ProcDir) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	return pd.NewStat()
}

func (pd *ProcDir) NewStat() (*sp.Stat, *serr.Err) {
	st := sp.NewStat(sp.NewQid(sp.QTDIR, 0, pd.path), pd.perm, 0, pd.name, "schedd")
	st.SetLengthInt(pd.procs.Len())
	st.SetMtime(time.Now().Unix())
	return st, nil
}

func (pd *ProcDir) LookupPath(ctx fs.CtxI, path path.Tpathname) ([]fs.FsObj, fs.FsObj, path.Tpathname, *serr.Err) {
	name := path[0]
	db.DPrintf(db.PROCFS, "%v: Lookup %v %v\n", ctx, pd, name)
	if p, ok := pd.procs.Lookup(name); ok {
		var o fs.FsObj
		pi := newProcInode(0444, name)
		pi.proc = p
		o = pi
		return []fs.FsObj{o}, o, path[1:], nil
	} else {
		return nil, nil, path, serr.NewErr(serr.TErrNotfound, name)
	}
}

func (pd *ProcDir) ReadDir(ctx fs.CtxI, cursor int, cnt sp.Tsize) ([]*sp.Stat, *serr.Err) {
	db.DPrintf(db.PROCFS, "%v: ReadDir %v %v %v\n", ctx, pd, cursor, cnt)
	dents := make([]*sp.Stat, 0, pd.sd.Len())
	pd.sd.Iter(func(n string, st *sp.Stat) bool {
		dents = append(dents, st)
		return true
	})
	if cursor > len(dents) {
		return nil, nil
	} else {
		return dents[cursor:], nil
	}
}
