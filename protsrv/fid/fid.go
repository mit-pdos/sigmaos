package fid

import (
	"fmt"
	"sync"

	"sigmaos/protsrv/pobj"
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

func NewFid(pobj *pobj.Pobj, m sp.Tmode, qid *sp.Tqid) *Fid {
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