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
	deadlock.Mutex
	isClosed bool
	c        *sync.Cond
}

type SessCond struct {
	// lock  *sync.Mutex
	lock  *deadlock.Mutex // e.g., pipe lock, watch lock
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

// Caller should hold sess lock and sc lock and will receive them back
// on return. Wait release sess lock, so that other threads on the
// session can run.
func (sc *SessCond) Wait(sessid np.Tsession) error {
	c := sc.alloc(sessid)
	c.Lock()

	// release sc lock and sess lock; c.Lock ensure atomicity of
	// releasing and going to sleep.
	sc.lock.Unlock()
	sc.st.WaitStart(sessid)

	c.c.Wait()

	closed := c.isClosed

	c.Unlock()

	// reacquire sess lock and sc lock
	sc.st.WaitDone(sessid)
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

// Close all sess conds, which wakes up waiting threads.  They will
// delete their sess conds from sct, so we need a lock around
// sct.conds.  But, DeleteSess violates lock order: lock sc.lock first
// (e.g., watch on directory), then sct.lock (if file watch must
// create sess cond in sct).  So, make copy first, then close()
// entries.  Threads many add new sc conds while bailing out (e.g., to
// remove an emphemeral file), but threads shouldn't wait.
func (sct *SessCondTable) DeleteSess(sessid np.Tsession) {
	t := sct.toSlice()
	//log.Printf("%v: delete sess %v\n", sessid, t)
	for _, sc := range t {
		sc.closed(sessid)
	}
}
