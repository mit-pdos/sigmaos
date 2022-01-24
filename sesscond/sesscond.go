package sesscond

import (
	"fmt"
	"log"
	"sync"
	// "errors"

	"github.com/sasha-s/go-deadlock"

	// db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/session"
)

//
// sesscond wraps cond vars so that if a session terminates, it can
// wakeup threads that are associated with that session.  Each cond
// var is represented as several cond vars, one per session.
//

type cond struct {
	deadlock.Mutex // to atomically release sess lock and sc lock before waiting on c
	isClosed       bool
	c              *sync.Cond
}

// Sess cond has one cond per session.  The lock is, for example, a
// pipe lock or watch lock, which SessCond releases in Wait() and
// re-acquires before returning out of Wait().
type SessCond struct {
	// lock  *sync.Mutex
	lock  *deadlock.Mutex
	st    *session.SessionTable
	nref  int // under sct lock
	conds map[np.Tsession]*cond
}

func makeSessCond(st *session.SessionTable, lock *deadlock.Mutex) *SessCond {
	sc := &SessCond{}
	sc.lock = lock
	sc.st = st
	sc.conds = make(map[np.Tsession]*cond)
	return sc
}

// A session has been closed: wake up threads associated with this
// session. Grab c lock to ensure that wakeup isn't missed while a
// thread is about enter wait (and releasing sess and sc lock).
func (sc *SessCond) closed(sessid np.Tsession) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	// log.Printf("cond %p: close %v %v\n", sc, sessid, sc.conds)
	if c, ok := sc.conds[sessid]; ok {
		// log.Printf("%p: sess %v closed\n", sc, sessid)
		c.Lock()
		c.isClosed = true
		c.c.Broadcast()
		c.Unlock()
	}
}

func (sc *SessCond) alloc(sessid np.Tsession) *cond {
	if c, ok := sc.conds[sessid]; ok {
		return c
	}
	c := &cond{}
	c.c = sync.NewCond(&c.Mutex)
	sc.conds[sessid] = c
	return c
}

// Caller should hold sess and sc lock and will receive them back on
// return. Wait releases sess lock, so that other threads on the
// session can run.  c.Lock ensure atomicity of releasing sess and sc
// lock and going to sleep.
func (sc *SessCond) Wait(sessid np.Tsession) error {
	c := sc.alloc(sessid)
	c.Lock()

	// release sc lock and sess lock
	sc.lock.Unlock()
	sc.st.SessUnlock(sessid)

	c.c.Wait()

	closed := c.isClosed

	c.Unlock()

	// reacquire sess lock and sc lock
	sc.st.SessLock(sessid)
	sc.lock.Lock()

	if closed {
		log.Printf("wait sess closed %v\n", sessid)
		return fmt.Errorf("session closed %v", sessid)
	}
	return nil
}

func (sc *SessCond) Signal() {
	for _, c := range sc.conds {
		// acquire c.lock() to ensure signal doesn't happen
		// between releasing sc or sess lock and going to
		// sleep.
		c.Lock()
		c.c.Signal()
		c.Unlock()
	}
}

func (sc *SessCond) Broadcast() {
	for _, c := range sc.conds {
		// See comment above
		c.Lock()
		c.c.Broadcast()
		c.Unlock()
	}
}

type SessCondTable struct {
	deadlock.Mutex
	// sync.Mutex
	conds  map[*SessCond]bool
	st     *session.SessionTable
	closed bool
}

func MakeSessCondTable(st *session.SessionTable) *SessCondTable {
	t := &SessCondTable{}
	t.conds = make(map[*SessCond]bool)
	t.st = st
	return t
}

func (sct *SessCondTable) MakeSessCond(lock *deadlock.Mutex) *SessCond {
	sct.Lock()
	defer sct.Unlock()

	sc := makeSessCond(sct.st, lock)
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
	//log.Printf("%v: delete sess %v\n", sessid, t)
	for _, sc := range t {
		sc.closed(sessid)
	}
}
