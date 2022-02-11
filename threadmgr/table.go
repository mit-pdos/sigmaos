package threadmgr

import (
	np "ulambda/ninep"
)

type ProcessFn func(fc *np.Fcall, replies chan *np.Fcall)

type ThreadTable struct {
	pfn     ProcessFn
	threads map[*Thread]bool
}

func MakeThreadTable(pfn ProcessFn) *ThreadTable {
	tm := &ThreadTable{}
	tm.pfn = pfn
	tm.threads = make(map[*Thread]bool)
	return tm
}

func (tm *ThreadTable) AddThread() *Thread {
	new := makeThread(tm.pfn)
	tm.threads[new] = true
	new.start()
	return new
}

func (tm *ThreadTable) RemoveThread(t *Thread) {
	t.stop()
	delete(tm.threads, t)
}
