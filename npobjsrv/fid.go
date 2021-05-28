package npobjsrv

import (
	"fmt"
	"log"
	"sync"

	np "ulambda/ninep"
	"ulambda/npcodec"
)

type Fid struct {
	mu   sync.Mutex
	path []string
	obj  NpObj
	vers np.TQversion
	ctx  CtxI
}

func (f *Fid) String() string {
	return fmt.Sprintf("p %v", f.path)
}

func (f *Fid) Ctx() CtxI {
	return f.ctx
}

func (f *Fid) Path() []string {
	return f.path
}

func (f *Fid) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.obj = nil
}

func (f *Fid) Obj() NpObj {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.obj
}

func (f *Fid) Write(off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	o := f.Obj()
	if o == nil {
		return 0, fmt.Errorf("Closed by server")
	}
	switch i := o.(type) {
	case NpObjFile:
		return i.Write(f.ctx, off, b, v)
	case NpObjDir:
		return i.WriteDir(f.ctx, off, b, v)
	default:
		log.Fatalf("Write: obj type %T isn't NpObjDir or NpObjFile\n", o)
		return 0, nil
	}
}

func (f *Fid) readDir(o NpObj, off np.Toffset, count np.Tsize, v np.TQversion, rets *np.Rread) *np.Rerror {
	var dirents []*np.Stat
	var err error
	if o.Size() > 0 && off >= np.Toffset(o.Size()) {
		dirents = []*np.Stat{}
	} else {
		d := o.(NpObjDir)
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
		return &np.Rerror{"Closed by server"}
	}
	switch i := o.(type) {
	case NpObjDir:
		return f.readDir(o, off, count, v, rets)
	case NpObjFile:
		b, err := i.Read(f.ctx, off, count, v)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		rets.Data = b
		return nil
	default:
		log.Fatalf("Read: obj type %T isn't NpObjDir or NpObjFile\n", o)
		return nil
	}
}
