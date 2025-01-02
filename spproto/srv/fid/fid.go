package fid

import (
	"fmt"
	"sync"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

// Several fids may name the same obj. For example, several clients
// may walk to the same file and have an fid for that file.
type Fid struct {
	mu     sync.Mutex
	obj    fs.FsObj // the obj in the backing file system
	name   string   // name for obj (for debugging and error messages)
	dir    fs.Dir   // parent dir of obj
	ctx    fs.CtxI  // the context of the attached sesssion
	isOpen bool     // has Create/Open() been called for po.Obj?
	m      sp.Tmode // mode for Create/Open()
	qid    sp.Tqid  // the qid of obj at the time of invoking NewFid
	cursor int      // for directories
}

func (f *Fid) String() string {
	return fmt.Sprintf("{n %q o %v dir %v o? %v m %v qid %v ctx %v}", f.name, f.obj, f.dir, f.isOpen, f.m, f.qid, f.ctx)
}

func (f *Fid) Obj() fs.FsObj {
	return f.obj
}

func (f *Fid) SetObj(o fs.FsObj) {
	f.obj = o
}

func (f *Fid) Name() string {
	return f.name
}

func (f *Fid) SetName(name string) {
	f.name = name
}

func (f *Fid) Path() sp.Tpath {
	return f.obj.Path()
}

func (f *Fid) Ctx() fs.CtxI {
	return f.ctx
}

func (f *Fid) Mode() sp.Tmode {
	return f.m
}

func (f *Fid) SetMode(m sp.Tmode) {
	f.isOpen = true
	f.m = m
}

func (f *Fid) Parent() fs.Dir {
	return f.dir
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
	var err *serr.Err
	sz := sp.Tsize(0)

	switch i := f.obj.(type) {
	case fs.File:
		sz, err = i.Write(f.ctx, off, b, fence)
	default:
		db.DFatalf("Write: obj type %T isn't Dir or File\n", f.obj)
	}
	return sz, err
}

func (f *Fid) WriteRead(req sessp.IoVec) (sessp.IoVec, *serr.Err) {
	var err *serr.Err
	var iov sessp.IoVec
	switch i := f.obj.(type) {
	case fs.RPC:
		iov, err = i.WriteRead(f.ctx, req)
	default:
		db.DFatalf("Write: obj type %T isn't RPC\n", f.obj)
	}
	return iov, err
}

func (f *Fid) readDir(o fs.FsObj, off sp.Toffset, count sp.Tsize) ([]byte, *serr.Err) {
	d := o.(fs.Dir)
	dirents, err := d.ReadDir(f.ctx, f.cursor, count)
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
	switch i := f.obj.(type) {
	case fs.Dir:
		return f.readDir(f.obj, off, count)
	case fs.File:
		b, err := i.Read(f.ctx, off, count, fence)
		if err != nil {
			return nil, err
		}
		return b, nil
	default:
		db.DFatalf("Read: obj %v type %T isn't Dir or File\n", f.obj, f.obj)
		return nil, nil
	}
}
