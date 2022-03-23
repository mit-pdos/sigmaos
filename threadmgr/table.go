package threadmgr

import (
	np "ulambda/ninep"
)

type ProcessFn func(fc *np.Fcall)

type ThreadMgrTable struct {
	pfn        ProcessFn
	threadmgrs map[*ThreadMgr]bool
	replicated bool
}

func MakeThreadMgrTable(pfn ProcessFn, replicated bool) *ThreadMgrTable {
	tm := &ThreadMgrTable{}
	tm.pfn = pfn
	tm.threadmgrs = make(map[*ThreadMgr]bool)
	tm.replicated = replicated
	return tm
}

func (tm *ThreadMgrTable) AddThread() *ThreadMgr {
	var new *ThreadMgr
	if tm.replicated && len(tm.threadmgrs) > 0 {
		for t, _ := range tm.threadmgrs {
			new = t
			break
		}
	} else {
		new = makeThreadMgr(tm.pfn)
		tm.threadmgrs[new] = true
		new.start()
	}
	return new
}

func (tm *ThreadMgrTable) RemoveThread(t *ThreadMgr) {
	if !tm.replicated {
		t.stop()
		delete(tm.threadmgrs, t)
	}
}

func (tmt *ThreadMgrTable) Snapshot() []byte {
	return tmt.snapshot()
}
