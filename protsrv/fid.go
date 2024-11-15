package protsrv

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/protsrv/pobj"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Fid struct {
	mu     sync.Mutex
	isOpen bool
	po     *pobj.Pobj
	m      sp.Tmode
	qid    *sp.Tqid // the qid of obj at the time of invoking NewFidPath
	cursor int      // for directories
}

func newFidPath(pobj *pobj.Pobj, m sp.Tmode, qid *sp.Tqid) *Fid {
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

func (f *Fid) Pobj() *pobj.Pobj {
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
	case *WatchFsObj:
		return i.GetEventBuffer(int(count)), nil
	default:
		db.DFatalf("Read: obj %v type %T isn't Dir or File or Watch\n", po.Obj(), po.Obj())
		return nil, nil
	}
}
