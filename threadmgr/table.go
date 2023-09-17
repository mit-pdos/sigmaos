package threadmgr

import (
	"sigmaos/sessp"
)

type ProcessFn func(fc *sessp.FcallMsg)

type ThreadMgrTable struct {
	pfn        ProcessFn
	threadmgrs map[*ThreadMgr]bool
}

func NewThreadMgrTable(pfn ProcessFn) *ThreadMgrTable {
	tm := &ThreadMgrTable{}
	tm.pfn = pfn
	tm.threadmgrs = make(map[*ThreadMgr]bool)
	return tm
}

func (tm *ThreadMgrTable) AddThread() *ThreadMgr {
	var new *ThreadMgr
	new = newThreadMgr(tm.pfn)
	tm.threadmgrs[new] = true
	new.start()
	return new
}

func (tm *ThreadMgrTable) RemoveThread(t *ThreadMgr) {
	t.stop()
	delete(tm.threadmgrs, t)
}
