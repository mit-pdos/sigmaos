package fid

import (
	"fmt"
	"sync"

	"sigmaos/api/fs"
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

func (f *Fid) Cursor() int {
	return f.cursor
}

func (f *Fid) IncCursor(n int) {
	f.cursor += n
}

func (f *Fid) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isOpen = false
}
