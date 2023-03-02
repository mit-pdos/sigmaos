package fid

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Pobj struct {
	path path.Path
	obj  fs.FsObj
	ctx  fs.CtxI
}

func MkPobj(p path.Path, o fs.FsObj, ctx fs.CtxI) *Pobj {
	return &Pobj{p, o, ctx}
}

func (po *Pobj) Path() path.Path {
	return po.path
}

func (po *Pobj) Ctx() fs.CtxI {
	return po.ctx
}

func (po *Pobj) SetPath(path path.Path) {
	po.path = path
}

func (po *Pobj) Obj() fs.FsObj {
	return po.obj
}

func (po *Pobj) SetObj(o fs.FsObj) {
	po.obj = o
}

type Fid struct {
	mu     sync.Mutex
	isOpen bool
	po     *Pobj
	m      sp.Tmode
	qid    *sp.Tqid // the qid of obj at the time of invoking MakeFidPath
	cursor int      // for directories
}

func MakeFidPath(pobj *Pobj, m sp.Tmode, qid *sp.Tqid) *Fid {
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

func (f *Fid) IsOpen() bool {
	return f.isOpen
}

func (f *Fid) Qid() *sp.Tqid {
	return f.qid
}

func (f *Fid) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isOpen = false
}

func (f *Fid) Write(off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	o := f.Pobj().Obj()
	var err *serr.Err
	sz := sessp.Tsize(0)

	switch i := o.(type) {
	case fs.File:
		sz, err = i.Write(f.Pobj().Ctx(), off, b, v)
	case fs.Dir:
		sz, err = i.WriteDir(f.Pobj().Ctx(), off, b, v)
	default:
		db.DFatalf("Write: obj type %T isn't Dir or File\n", o)
	}
	return sz, err
}

func (f *Fid) WriteRead(req []byte) ([]byte, *serr.Err) {
	o := f.Pobj().Obj()
	var err *serr.Err
	var b []byte
	switch i := o.(type) {
	case fs.RPC:
		b, err = i.WriteRead(f.Pobj().Ctx(), req)
	default:
		db.DFatalf("Write: obj type %T isn't RPC\n", o)
	}
	return b, err
}

func (f *Fid) readDir(o fs.FsObj, off sp.Toffset, count sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	d := o.(fs.Dir)
	dirents, err := d.ReadDir(f.Pobj().Ctx(), f.cursor, count, v)
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

func (f *Fid) Read(off sp.Toffset, count sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	po := f.Pobj()
	switch i := po.Obj().(type) {
	case fs.Dir:
		return f.readDir(po.Obj(), off, count, v)
	case fs.File:
		b, err := i.Read(po.Ctx(), off, count, v)
		if err != nil {
			return nil, err
		}
		return b, nil
	default:
		db.DFatalf("Read: obj %v type %T isn't Dir or File\n", po.Obj(), po.Obj())
		return nil, nil
	}
}
