package fid

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/sigmap"
    "sigmaos/fcall"
	"sigmaos/spcodec"
)

type Pobj struct {
	path np.Path
	obj  fs.FsObj
	ctx  fs.CtxI
}

func MkPobj(p np.Path, o fs.FsObj, ctx fs.CtxI) *Pobj {
	return &Pobj{p, o, ctx}
}

func (po *Pobj) Path() np.Path {
	return po.path
}

func (po *Pobj) Ctx() fs.CtxI {
	return po.ctx
}

func (po *Pobj) SetPath(path np.Path) {
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
	m      np.Tmode
	qid    np.Tqid // the qid of obj at the time of invoking MakeFidPath
	cursor int     // for directories
}

func MakeFidPath(pobj *Pobj, m np.Tmode, qid np.Tqid) *Fid {
	return &Fid{sync.Mutex{}, false, pobj, m, qid, 0}
}

func (f *Fid) String() string {
	return fmt.Sprintf("po %v o? %v %v v %v", f.po, f.isOpen, f.m, f.qid)
}

func (f *Fid) Mode() np.Tmode {
	return f.m
}

func (f *Fid) SetMode(m np.Tmode) {
	f.isOpen = true
	f.m = m
}

func (f *Fid) Pobj() *Pobj {
	return f.po
}

func (f *Fid) IsOpen() bool {
	return f.isOpen
}

func (f *Fid) Qid() np.Tqid {
	return f.qid
}

func (f *Fid) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isOpen = false
}

func (f *Fid) Write(off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *fcall.Err) {
	o := f.Pobj().Obj()
	var err *fcall.Err
	sz := np.Tsize(0)

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

func (f *Fid) WriteRead(req []byte) ([]byte, *fcall.Err) {
	o := f.Pobj().Obj()
	var err *fcall.Err
	var b []byte
	switch i := o.(type) {
	case fs.RPC:
		b, err = i.WriteRead(f.Pobj().Ctx(), req)
	default:
		db.DFatalf("Write: obj type %T isn't RPC\n", o)
	}
	return b, err
}

func (f *Fid) readDir(o fs.FsObj, off np.Toffset, count np.Tsize, v np.TQversion, rets *np.Rread) *fcall.Err {
	d := o.(fs.Dir)
	dirents, err := d.ReadDir(f.Pobj().Ctx(), f.cursor, count, v)
	if err != nil {
		return err
	}
	b, n, err := spcodec.MarshalDir(count, dirents)
	if err != nil {
		return err
	}
	f.cursor += n
	rets.Data = b
	return nil
}

func (f *Fid) Read(off np.Toffset, count np.Tsize, v np.TQversion, rets *np.Rread) *fcall.Err {
	po := f.Pobj()
	switch i := po.Obj().(type) {
	case fs.Dir:
		return f.readDir(po.Obj(), off, count, v, rets)
	case fs.File:
		b, err := i.Read(po.Ctx(), off, count, v)
		if err != nil {
			return err
		}
		rets.Data = b
		return nil
	default:
		db.DFatalf("Read: obj %v type %T isn't Dir or File\n", po.Obj(), po.Obj())
		return nil
	}
}
