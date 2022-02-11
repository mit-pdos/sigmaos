package threadmgr

import (
	np "ulambda/ninep"
)

type ProcessFn func(fc *np.Fcall, replies chan *np.Fcall)

type ThreadMgrTable struct {
	pfn     ProcessFn
	threads map[*Thread]bool
}

func MakeThreadMgrTable(pfn ProcessFn) *ThreadMgrTable {
	tm := &ThreadMgrTable{}
	tm.pfn = pfn
	tm.threads = make(map[*Thread]bool)
	return tm
}

func (tm *ThreadMgrTable) AddThread() *Thread {
	new := makeThread(tm.pfn)
	tm.threads[new] = true
	new.start()
	return new
}

func (tm *ThreadMgrTable) RemoveThread(t *Thread) {
	t.stop()
	delete(tm.threads, t)
}
