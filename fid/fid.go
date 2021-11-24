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
	mu   sync.Mutex
	path []string
	obj  fs.FsObj
	m    np.Tmode
	vers np.TQversion
	ctx  fs.CtxI
}

func MakeFidPath(p []string, o fs.FsObj, m np.Tmode, ctx fs.CtxI) *Fid {
	return &Fid{sync.Mutex{}, p, o, m, o.Version(), ctx}
}

func (f *Fid) String() string {
	return fmt.Sprintf("p %v m %v v %v", f.path, f.m, f.vers)
}

func (f *Fid) Ctx() fs.CtxI {
	return f.ctx
}

func (f *Fid) Mode() np.Tmode {
	return f.m
}

func (f *Fid) SetMode(m np.Tmode) {
	f.m = m
}

func (f *Fid) Path() []string {
	return f.path
}

func (f *Fid) SetPath(p []string) {
	f.path = p

}

func (f *Fid) PathLast() string {
	return f.path[len(f.path)-1]
}

func (f *Fid) PathDir() []string {
	return f.path[:len(f.path)-1]
}

func (f *Fid) ObjU() fs.FsObj {
	return f.obj
}

func (f *Fid) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.obj = nil
}

func (f *Fid) Obj() fs.FsObj {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.obj
}

func (f *Fid) SetObj(o fs.FsObj) {
	f.obj = o
}

func (f *Fid) Write(off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Rerror) {
	o := f.Obj()
	if o == nil {
		return 0, np.ErrClunked
	}
	var err error
	sz := np.Tsize(0)
	switch i := o.(type) {
	case fs.File:
		sz, err = i.Write(f.ctx, off, b, v)
	case fs.Dir:
		sz, err = i.WriteDir(f.ctx, off, b, v)
	default:
		log.Fatalf("Write: obj type %T isn't Dir or File\n", o)
	}
	var r *np.Rerror
	if err != nil {
		r = &np.Rerror{err.Error()}
	}
	return sz, r
}

func (f *Fid) readDir(o fs.FsObj, off np.Toffset, count np.Tsize, v np.TQversion, rets *np.Rread) *np.Rerror {
	var dirents []*np.Stat
	var err error
	if o.Size() > 0 && off >= np.Toffset(o.Size()) {
		dirents = []*np.Stat{}
	} else {
		d := o.(fs.Dir)
		dirents, err = d.ReadDir(f.ctx, off, count, v)

	}
	b, err := npcodec.Dir2Byte(off, count, dirents)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

func (f *Fid) Read(off np.Toffset, count np.Tsize, v np.TQversion, rets *np.Rread) *np.Rerror {
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	switch i := o.(type) {
	case fs.Dir:
		return f.readDir(o, off, count, v, rets)
	case fs.File:
		b, err := i.Read(f.ctx, off, count, v)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		rets.Data = b
		return nil
	default:
		log.Fatalf("Read: obj type %T isn't Dir or File\n", o)
		return nil
	}
}
