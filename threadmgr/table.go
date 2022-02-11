package threadmgr

import (
	np "ulambda/ninep"
)

type ProcessFn func(fc *np.Fcall, replies chan *np.Fcall)

type ThreadMgrTable struct {
	pfn         ProcessFn
	threadmgrss map[*ThreadMgr]bool
}

func MakeThreadMgrTable(pfn ProcessFn) *ThreadMgrTable {
	tm := &ThreadMgrTable{}
	tm.pfn = pfn
	tm.threadmgrss = make(map[*ThreadMgr]bool)
	return tm
}

func (tm *ThreadMgrTable) AddThread() *ThreadMgr {
	new := makeThreadMgr(tm.pfn)
	tm.threadmgrss[new] = true
	new.start()
	return new
}

func (tm *ThreadMgrTable) RemoveThread(t *ThreadMgr) {
	t.stop()
	delete(tm.threadmgrss, t)
}
