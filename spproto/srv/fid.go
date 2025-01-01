package srv

import (
	"fmt"
	"sync"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

// Several fids may name the same Pobj. For example, each session's
// fid 0 refers to the root of the file system.
type Pobj struct {
	pathname path.Tpathname
	obj      fs.FsObj
	parent   fs.Dir
	ctx      fs.CtxI
}

func newPobj(pn path.Tpathname, o fs.FsObj, dir fs.Dir, ctx fs.CtxI) *Pobj {
	return &Pobj{pathname: pn, parent: dir, obj: o, ctx: ctx}
}

func (po *Pobj) String() string {
	return fmt.Sprintf("{pn '%v'(p %d) o %v parent %v ctx %v}", po.pathname, po.Path(), po.obj, po.parent, po.ctx)
}

func (po *Pobj) Pathname() path.Tpathname {
	return po.pathname
}

func (po *Pobj) Path() sp.Tpath {
	return po.obj.Path()
}

func (po *Pobj) Ctx() fs.CtxI {
	return po.ctx
}

func (po *Pobj) SetPath(path path.Tpathname) {
	po.pathname = path
}

func (po *Pobj) Obj() fs.FsObj {
	return po.obj
}

func (po *Pobj) SetObj(o fs.FsObj) {
	po.obj = o
}

func (po *Pobj) Parent() fs.Dir {
	return po.parent
}

type Fid struct {
	mu     sync.Mutex
	isOpen bool
	po     *Pobj
	m      sp.Tmode
	qid    sp.Tqid // the qid of obj at the time of invoking NewFidPath
	cursor int     // for directories
}

func newFidPath(pobj *Pobj, m sp.Tmode, qid sp.Tqid) *Fid {
	return &Fid{sync.Mutex{}, false, pobj, m, qid, 0}
}

func (f *Fid) String() string {
	return fmt.Sprintf("{po %v o? %v %v v %v}", f.po, f.isOpen, f.m, f.qid)
}

func (f *Fid) Mode() sp.Tmode {
	return f.m
}

func (f *Fid) SetMode(m sp.Tmode) {
	f.isOpen = true
	f.m = m
}

func (f *Fid) Pobj() *Pobj {
	return f.po
}

func (f *Fid) Parent() fs.Dir {
	return f.po.parent
}

func (f *Fid) IsOpen() bool {
	return f.isOpen
}

func (f *Fid) Qid() *sp.Tqid {
	return &f.qid
}

func (f *Fid) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isOpen = false
}

func (f *Fid) Write(off sp.Toffset, b []byte, fence sp.Tfence) (sp.Tsize, *serr.Err) {
	o := f.Pobj().Obj()
	var err *serr.Err
	sz := sp.Tsize(0)

	switch i := o.(type) {
	case fs.File:
		sz, err = i.Write(f.Pobj().Ctx(), off, b, fence)
	default:
		db.DFatalf("Write: obj type %T isn't Dir or File\n", o)
	}
	return sz, err
}

func (f *Fid) WriteRead(req sessp.IoVec) (sessp.IoVec, *serr.Err) {
	o := f.Pobj().Obj()
	var err *serr.Err
	var iov sessp.IoVec
	switch i := o.(type) {
	case fs.RPC:
		iov, err = i.WriteRead(f.Pobj().Ctx(), req)
	default:
		db.DFatalf("Write: obj type %T isn't RPC\n", o)
	}
	return iov, err
}

func (f *Fid) readDir(o fs.FsObj, off sp.Toffset, count sp.Tsize) ([]byte, *serr.Err) {
	d := o.(fs.Dir)
	dirents, err := d.ReadDir(f.Pobj().Ctx(), f.cursor, count)
	if err != nil {
		return nil, err
	}
	b, n, err := fs.MarshalDir(count, dirents)
	if err != nil {
		return nil, err
	}
	f.cursor += n
	return b, nil
}

func (f *Fid) Read(off sp.Toffset, count sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
	po := f.Pobj()
	switch i := po.Obj().(type) {
	case fs.Dir:
		return f.readDir(po.Obj(), off, count)
	case fs.File:
		b, err := i.Read(po.Ctx(), off, count, fence)
		if err != nil {
			return nil, err
		}
		return b, nil
	default:
		db.DFatalf("Read: obj %v type %T isn't Dir or File\n", po.Obj(), po.Obj())
		return nil, nil
	}
}
