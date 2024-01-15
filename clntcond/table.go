package clntcond

import (
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type ClntCondTable struct {
	//	deadlock.Mutex
	sync.Mutex
	conds  map[*ClntCond]bool
	closed bool
}

func NewClntCondTable() *ClntCondTable {
	t := &ClntCondTable{}
	t.conds = make(map[*ClntCond]bool)
	return t
}

func (sct *ClntCondTable) NewClntCond(lock sync.Locker) *ClntCond {
	sct.Lock()
	defer sct.Unlock()

	sc := newClntCond(sct, lock)
	sct.conds[sc] = true
	sc.nref++
	return sc
}

func (sct *ClntCondTable) FreeClntCond(sc *ClntCond) {
	sct.Lock()
	defer sct.Unlock()
	sc.nref--
	if sc.nref != 0 {
		db.DFatalf("FreeClntCond %v\n", sc)
	}
	delete(sct.conds, sc)
}

func (sct *ClntCondTable) toSlice() []*ClntCond {
	sct.Lock()
	defer sct.Unlock()

	sct.closed = true
	t := make([]*ClntCond, 0, len(sct.conds))
	for sc, _ := range sct.conds {
		t = append(t, sc)
	}
	return t
}

// Close all clnt conds for cid, which wakes up waiting threads.  A
// thread may delete a clnt cond from sct, if the thread is the last
// user.  So we need, a lock around sct.conds.  But, DeleteClnt
// violates lock order, which is lock sc.lock first (e.g., watch on
// directory), then acquire sct.lock (if file watch must create sess
// cond in sct).  To avoid order violation, DeleteClnt news copy
// first, then close() clnt conds.  Threads many add new clnt conds to
// sct while bailing out (e.g., to remove an emphemeral file), but
// threads shouldn't wait on these clnt conds, so we don't have to
// close those.
func (sct *ClntCondTable) DeleteClnt(cid sp.TclntId) {
	t := sct.toSlice()
	db.DPrintf(db.CLNTCOND, "DeleteClnt cid %v %v\n", cid, t)
	for _, sc := range t {
		sc.closed(cid)
	}
}
