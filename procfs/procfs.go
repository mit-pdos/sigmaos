package procfs

import (
	"fmt"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ProcFs interface {
	GetProcs() []*proc.Proc
	Lookup(string) (*proc.Proc, bool)
	Len() int
}

type ProcInode struct {
	path   sp.Tpath
	perm   sp.Tperm // XXX kill, but requires changing Perm() API
	name   string
	parent fs.Dir
	proc   *proc.Proc
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

func (pi *ProcInode) IsLeased() bool {
	return false
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

func (pi *ProcInode) Write(fs.CtxI, sp.Toffset, []byte, sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, serr.NewErr(serr.TErrNotSupported, "Write")
}

func (pi *ProcInode) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
	s := pi.proc.String()
	if off > sp.Toffset(len(s)) {
		return nil, nil
	}
	return []byte(s)[off:], nil
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
	st.SetLength(1)
	st.SetMtime(time.Now().Unix())
	return st, nil
}
