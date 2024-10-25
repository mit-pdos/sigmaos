package protsrv

import (
	"fmt"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sync"
)

type FidWatch struct {
	mu     *sync.Mutex
	po     *Pobj
	qid    *sp.Tqid // the qid of obj at the time of invoking NewFidWatch
	events []string
	cond   sync.Cond
	watch  *WatchV2
}

func newFidWatch(pobj *Pobj, qid *sp.Tqid) *FidWatch {
	mu := sync.Mutex{}
	db.DPrintf(db.WATCH_NEW, "NewFidWatch '%v'\n", pobj.Pathname())
	return &FidWatch{&mu, pobj, qid, nil, *sync.NewCond(&mu), nil}
}

func (f *FidWatch) String() string {
	return fmt.Sprintf("{po %v v %v ev %v}", f.po, f.qid, f.events)
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

// gets events and clears the event list, blocking if there are currently no events
func (f *FidWatch) GetEvents() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	for f.events == nil {
		f.cond.Wait()
	}

	ret := f.events
	f.events = nil
	return ret
}
