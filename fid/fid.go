package fid

import (
	"fmt"
	"log"
	"sync"

	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type Fid struct {
	mu     sync.Mutex
	path   []string
	obj    fs.FsObj
	isOpen bool
	m      np.Tmode
	vers   np.TQversion
	ctx    fs.CtxI
	cursor int // for directories
}

func MakeFidPath(p []string, o fs.FsObj, m np.Tmode, ctx fs.CtxI) *Fid {
	return &Fid{sync.Mutex{}, p, o, false, m, o.Version(), ctx, 0}
}

func (f *Fid) String() string {
	return fmt.Sprintf("p %v o? %v %v v %v", f.path, f.isOpen, f.m, f.vers)
}

func (f *Fid) Ctx() fs.CtxI {
	return f.ctx
}

func (f *Fid) Mode() np.Tmode {
	return f.m
}

func (f *Fid) SetMode(m np.Tmode) {
	f.isOpen = true
	f.m = m
}

func (f *Fid) IsOpen() bool {
	return f.isOpen
}

func (f *Fid) Path() []string {
	return f.path
}

func (f *Fid) SetPath(p []string) {
	f.path = p

}

func (f *Fid) PathLast() string {
	return np.Base(f.path)
}

func (f *Fid) PathDir() []string {
	return np.Dir(f.path)
}

func (f *Fid) ObjU() fs.FsObj {
	return f.obj
}

func (f *Fid) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.obj = nil
	f.isOpen = false
}

func (f *Fid) Obj() fs.FsObj {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.obj
}

func (f *Fid) SetObj(o fs.FsObj) {
	f.obj = o
}

func (f *Fid) Write(off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	o := f.Obj()
	var err *np.Err
	sz := np.Tsize(0)
	switch i := o.(type) {
	case fs.File:
		sz, err = i.Write(f.ctx, off, b, v)
	case fs.Dir:
		sz, err = i.WriteDir(f.ctx, off, b, v)
	default:
		log.Fatalf("FATAL Write: obj type %T isn't Dir or File\n", o)
	}
	return sz, err
}

func (f *Fid) readDir(o fs.FsObj, off np.Toffset, count np.Tsize, v np.TQversion, rets *np.Rread) *np.Err {
	var dirents []*np.Stat
	if o.Size() > 0 && off >= np.Toffset(o.Size()) {
		dirents = []*np.Stat{}
	} else {
		var err *np.Err
		d := o.(fs.Dir)
		dirents, err = d.ReadDir(f.ctx, f.cursor, count, v)
		if err != nil {
			return err
		}
	}
	b, n, error := npcodec.Dir2Byte(count, dirents)
	if error != nil {
		return np.MkErr(np.TErrError, error)
	}
	f.cursor += n
	rets.Data = b
	return nil
}

func (f *Fid) Read(off np.Toffset, count np.Tsize, v np.TQversion, rets *np.Rread) *np.Err {
	o := f.Obj()
	switch i := o.(type) {
	case fs.Dir:
		return f.readDir(o, off, count, v, rets)
	case fs.File:
		b, err := i.Read(f.ctx, off, count, v)
		if err != nil {
			return err
		}
		rets.Data = b
		return nil
	default:
		log.Fatalf("FATAL Read: obj type %T isn't Dir or File\n", o)
		return nil
	}
}
