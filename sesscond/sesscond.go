package sesscond

import (
	"fmt"
	"log"
	"sync"
	// "errors"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/session"
	"ulambda/threadmgr"
)

//
// sesscond wraps cond vars so that if a session terminates, it can
// wakeup threads that are associated with that session.  Each cond
// var is represented as several cond vars, one per goroutine using it.
//

type cond struct {
	isClosed  bool
	threadmgr *threadmgr.ThreadMgr
	c         *sync.Cond
}

// Sess cond has one cond per session.  The lock is, for example, a
// pipe lock or watch lock, which SessCond releases in Wait() and
// re-acquires before returning out of Wait().
type SessCond struct {
	lock        sync.Locker
	sct         *SessCondTable
	nref        int // under sct lock
	conds       map[np.Tsession][]*cond
	wakingConds map[np.Tsession]map[*cond]bool // Conds pending wakeup (which also may need to be alerted that the session has closed)
}

func makeSessCond(sct *SessCondTable, lock sync.Locker) *SessCond {
	sc := &SessCond{}
	sc.sct = sct
	sc.lock = lock
	sc.conds = make(map[np.Tsession][]*cond)
	sc.wakingConds = make(map[np.Tsession]map[*cond]bool)
	return sc
}

func (sc *SessCond) alloc(sessid np.Tsession) *cond {
	if _, ok := sc.conds[sessid]; !ok {
		sc.conds[sessid] = []*cond{}
	}
	c := &cond{}
	c.threadmgr = sc.sct.St.SessThread(sessid)
	c.c = sync.NewCond(sc.lock)
	sc.conds[sessid] = append(sc.conds[sessid], c)
	return c
}

// Caller should hold sc lock and will receive it back on return. Wait releases
// sess lock, so that other threads on the session can run. sc.lock ensures
// atomicity of releasing sc lock and going to sleep.
func (sc *SessCond) Wait(sessid np.Tsession) *np.Err {
	c := sc.alloc(sessid)

	sess, _ := sc.sct.St.Lookup(sessid)
	c.threadmgr.Sleep(c.c, sess.SetRunning)

	sc.removeWakingCond(sessid, c)

	closed := c.isClosed

	if closed {
		db.DPrintf("SESSCOND", "wait sess closed %v\n", sessid)
		return np.MkErr(np.TErrClosed, fmt.Sprintf("session %v", sessid))
	}
	return nil
}

// Caller should hold sc lock.
func (sc *SessCond) Signal() {
	for sid, condlist := range sc.conds {
		// acquire c.lock() to ensure signal doesn't happen
		// between releasing sc or sess lock and going to
		// sleep.
		for _, c := range condlist {
			c.threadmgr.Wake(c.c)
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Caller should hold sc lock.
func (sc *SessCond) Broadcast() {
	for sid, condlist := range sc.conds {
		for _, c := range condlist {
			c.threadmgr.Wake(c.c)
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Keep track of conds which are waiting to be woken up, since their session
// may close in the interim.
func (sc *SessCond) addWakingCond(sid np.Tsession, c *cond) {
	if _, ok := sc.wakingConds[sid]; !ok {
		sc.wakingConds[sid] = make(map[*cond]bool)
	}
	sc.wakingConds[sid][c] = true
}

func (sc *SessCond) removeWakingCond(sid np.Tsession, c *cond) {
	delete(sc.wakingConds[sid], c)
	if len(sc.wakingConds[sid]) == 0 {
		delete(sc.wakingConds, sid)
	}
}

// A session has been closed: wake up threads associated with this
// session. Grab c lock to ensure that wakeup isn't missed while a
// thread is about enter wait (and releasing sess and sc lock).
func (sc *SessCond) closed(sessid np.Tsession) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	db.DPrintf("SESSCOND", "cond %p: close %v %v\n", sc, sessid, sc.conds)
	if condlist, ok := sc.conds[sessid]; ok {
		db.DPrintf("SESSCOND", "%p: sess %v closed\n", sc, sessid)
		for _, c := range condlist {
			c.threadmgr.Wake(c.c)
			sc.addWakingCond(sessid, c)
		}
		delete(sc.conds, sessid)
	}
	// Mark all this session's waking conds as closed.
	for c, _ := range sc.wakingConds[sessid] {
		c.isClosed = true
	}
}

type SessCondTable struct {
	//	deadlock.Mutex
	sync.Mutex
	conds  map[*SessCond]bool
	St     *session.SessionTable
	closed bool
}

func MakeSessCondTable(st *session.SessionTable) *SessCondTable {
	t := &SessCondTable{}
	t.conds = make(map[*SessCond]bool)
	t.St = st
	return t
}

func (sct *SessCondTable) MakeSessCond(lock sync.Locker) *SessCond {
	sct.Lock()
	defer sct.Unlock()

	sc := makeSessCond(sct, lock)
	sct.conds[sc] = true
	sc.nref++
	return sc
}

func (sct *SessCondTable) FreeSessCond(sc *SessCond) {
	sct.Lock()
	defer sct.Unlock()
	sc.nref--
	if sc.nref != 0 {
		log.Fatalf("freesesscond %v\n", sc)
	}
	delete(sct.conds, sc)
}

func (sct *SessCondTable) toSlice() []*SessCond {
	sct.Lock()
	defer sct.Unlock()

	sct.closed = true
	t := make([]*SessCond, 0, len(sct.conds))
	for sc, _ := range sct.conds {
		t = append(t, sc)
	}
	return t
}

// Close all sess conds for sessid, which wakes up waiting threads.  A
// thread may delete a sess cond from sct, if the thread is the last
// user.  So we need, a lock around sct.conds.  But, DeleteSess
// violates lock order, which is lock sc.lock first (e.g., watch on
// directory), then acquire sct.lock (if file watch must create sess
// cond in sct).  To avoid order violation, DeleteSess makes copy
// first, then close() sess conds.  Threads many add new sess conds to
// sct while bailing out (e.g., to remove an emphemeral file), but
// threads shouldn't wait on these sess conds, so we don't have to
// close those.
func (sct *SessCondTable) DeleteSess(sessid np.Tsession) {
	t := sct.toSlice()
	db.DPrintf("SESSCOND", "%v: delete sess %v\n", sessid, t)
	for _, sc := range t {
		sc.closed(sessid)
	}
}
