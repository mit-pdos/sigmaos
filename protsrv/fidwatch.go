package protsrv

import (
	"fmt"
	sp "sigmaos/sigmap"
	"sync"
)

type FidWatch struct {
	mu     sync.Mutex
	po     *Pobj
	qid    *sp.Tqid // the qid of obj at the time of invoking NewFidWatch
}

func newFidWatch(pobj *Pobj, qid *sp.Tqid) *FidWatch {
	return &FidWatch{sync.Mutex{}, pobj, qid}
}

func (f *FidWatch) String() string {
	return fmt.Sprintf("{po %v o? %v %v v %v}", f.po, f.qid)
}

func (f *FidWatch) Pobj() *Pobj {
	return f.po
}

func (f *FidWatch) Qid() *sp.Tqid {
	return f.qid
}

func (f *FidWatch) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
}

