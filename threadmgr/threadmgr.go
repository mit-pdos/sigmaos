package threadmgr

import (
	np "ulambda/ninep"
)

type ProcessFn func(fc *np.Fcall, replies chan *np.Fcall)

type ThreadMgr struct {
	pfn     ProcessFn
	threads map[*Thread]bool
}

func MakeThreadMgr(pfn ProcessFn) *ThreadMgr {
	tm := &ThreadMgr{}
	tm.pfn = pfn
	tm.threads = make(map[*Thread]bool)
	return tm
}

func (tm *ThreadMgr) AddThread() *Thread {
	new := makeThread(tm.pfn)
	tm.threads[new] = true
	new.start()
	return new
}

func (tm *ThreadMgr) RemoveThread(t *Thread) {
	t.stop()
	delete(tm.threads, t)
}
