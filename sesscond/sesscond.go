// package sesscond wraps cond vars so that if a client terminates, it
// can wakeup threads that are associated with that client.  Each cond
// var is represented as several cond vars, one per goroutine using
// it.
package sesscond

import (
	"fmt"
	"sync"
	// "errors"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type cond struct {
	isClosed bool
	c        *sync.Cond
}

// Sess cond has one cond per client.  The lock is, for example, a
// pipe lock or watch lock, which SessCond releases in Wait() and
// re-acquires before returning out of Wait().
type SessCond struct {
	lock        sync.Locker
	sct         *SessCondTable
	nref        int // under sct lock
	conds       map[sp.TclntId][]*cond
	wakingConds map[sp.TclntId]map[*cond]bool // Conds pending wakeup (which also may need to be alerted that the session has closed)
}

func newSessCond(sct *SessCondTable, lock sync.Locker) *SessCond {
	sc := &SessCond{}
	sc.sct = sct
	sc.lock = lock
	sc.conds = make(map[sp.TclntId][]*cond)
	sc.wakingConds = make(map[sp.TclntId]map[*cond]bool)
	return sc
}

func (sc *SessCond) alloc(sessid sp.TclntId) *cond {
	if _, ok := sc.conds[sessid]; !ok {
		sc.conds[sessid] = []*cond{}
	}
	c := &cond{c: sync.NewCond(sc.lock)}
	db.DPrintf(db.SESSCOND, "alloc sc %v %v c %v\n", sc, sessid, c)
	sc.conds[sessid] = append(sc.conds[sessid], c)
	return c
}

// Caller should hold sc lock and will receive it back on return. Wait
// releases sess lock, so that other threads associated with client
// can run. sc.lock ensures atomicity of releasing sc lock and going
// to sleep.
func (sc *SessCond) Wait(cid sp.TclntId) *serr.Err {
	c := sc.alloc(cid)

	db.DPrintf(db.SESSCOND, "Wait %v c %v\n", cid, c)

	c.c.Wait()

	sc.removeWakingCond(cid, c)

	closed := c.isClosed

	db.DPrintf(db.SESSCOND, "Wait return %v\n", c)

	if closed {
		db.DPrintf(db.SESSCOND, "wait sess closed %v\n", cid)
		return serr.NewErr(serr.TErrClosed, fmt.Sprintf("session %v", cid))
	}
	return nil
}

// Caller should hold sc lock and the sleeper should have gone to
// sleep while holding sc.lock and release it in inside of sleep, so
// it shouldn't be running anymore.
func (sc *SessCond) Signal() {
	for sid, condlist := range sc.conds {
		for _, c := range condlist {
			db.DPrintf(db.SESSCOND, "Signal %v c %v\n", sid, c)
			c.c.Signal()
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Caller should hold sc lock.
func (sc *SessCond) Broadcast() {
	for sid, condlist := range sc.conds {
		for _, c := range condlist {
			c.c.Signal()
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Keep track of conds which are waiting to be woken up, since their session
// may close in the interim.
func (sc *SessCond) addWakingCond(sid sp.TclntId, c *cond) {
	if _, ok := sc.wakingConds[sid]; !ok {
		sc.wakingConds[sid] = make(map[*cond]bool)
	}
	sc.wakingConds[sid][c] = true
}

func (sc *SessCond) removeWakingCond(sid sp.TclntId, c *cond) {
	delete(sc.wakingConds[sid], c)
	if len(sc.wakingConds[sid]) == 0 {
		delete(sc.wakingConds, sid)
	}
}

// A client has detached: wake up threads associated with this
// client. Grab c lock to ensure that wakeup isn't missed while a
// thread is about enter wait (and releasing sess and sc lock).
func (sc *SessCond) closed(cid sp.TclntId) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	db.DPrintf(db.SESSCOND, "cond %p: close %v %v\n", sc, cid, sc.conds)
	if condlist, ok := sc.conds[cid]; ok {
		db.DPrintf(db.SESSCOND, "%p: sess %v closed\n", sc, cid)
		for _, c := range condlist {
			c.c.Signal()
			sc.addWakingCond(cid, c)
		}
		delete(sc.conds, cid)
	}
	// Mark all this client's waking conds as closed.
	for c, _ := range sc.wakingConds[cid] {
		c.isClosed = true
	}
}
