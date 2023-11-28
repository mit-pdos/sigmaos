package procmgr

import (
	"fmt"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ProcInode struct {
	path   sp.Tpath
	perm   sp.Tperm // XXX kill, but requires changing Perm() API
	name   string
	parent fs.Dir
}

func newProcInode(perm sp.Tperm, name string) *ProcInode {
	return &ProcInode{perm: perm, name: name}
}

func (pi *ProcInode) String() string {
	return fmt.Sprintf("{%v, %v, %q}", pi.path, pi.perm, pi.name)
}

func (pi *ProcInode) Perm() sp.Tperm {
	return pi.perm
}

func (pi *ProcInode) Path() sp.Tpath {
	return pi.path
}

func (pi *ProcInode) Parent() fs.Dir {
	return pi.parent
}

func (pi *ProcInode) SetParent(p fs.Dir) {
}

func (pi *ProcInode) Unlink() {
}

func (pi *ProcInode) Mtime() int64 {
	return 0
}

func (pi *ProcInode) SetMtime(m int64) {
}

func (pi *ProcInode) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	return nil, nil
}

func (pi *ProcInode) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	return nil
}

func (pi *ProcInode) Size() (sp.Tlength, *serr.Err) {
	st, err := pi.Stat(nil)
	if err != nil {
		return 0, err
	}
	return st.Tlength(), nil
}

func (pi *ProcInode) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.PROCFS, "%v: Stat %v %T\n", ctx, pi, pi)
	st := sp.NewStat(sp.NewQid(sp.QTFILE, 0, pi.path), pi.perm, 0, pi.name, "schedd")
	st.Length = 1
	st.Mtime = uint32(time.Now().Unix())
	return st, nil
}

type ProcDir struct {
	sync.Mutex
	*ProcInode
	procs map[sp.Tpid]*proc.Proc
}

func NewProcDir(procs map[sp.Tpid]*proc.Proc) fs.Inode {
	return &ProcDir{ProcInode: newProcInode(sp.DMDIR|0444, "pids"), procs: procs}
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
	st := sp.NewStat(sp.NewQid(sp.QTDIR, 0, pd.path), pd.perm, 0, pd.name, "schedd")
	st.Length = uint64(len(pd.procs))
	st.Mtime = uint32(time.Now().Unix())
	return st, nil
}

func (pd *ProcDir) LookupPath(ctx fs.CtxI, path path.Path) ([]fs.FsObj, fs.FsObj, path.Path, *serr.Err) {
	name := path[0]
	db.DPrintf(db.PROCFS, "%v: Lookup %v %v\n", ctx, pd, name)
	if _, ok := pd.procs[sp.Tpid(name)]; ok {
		var o fs.FsObj
		o = newProcInode(0444, name)
		return []fs.FsObj{o}, o, path[1:], nil
	} else {
		return nil, nil, path, serr.NewErr(serr.TErrNotfound, name)
	}
}

func (pd *ProcDir) ReadDir(ctx fs.CtxI, cursor int, cnt sp.Tsize) ([]*sp.Stat, *serr.Err) {
	return nil, nil
}
