package sesscond

import (
	"fmt"
	"sync"
	// "errors"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/threadmgr"
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
	conds       map[sessp.Tsession][]*cond
	wakingConds map[sessp.Tsession]map[*cond]bool // Conds pending wakeup (which also may need to be alerted that the session has closed)
}

func makeSessCond(sct *SessCondTable, lock sync.Locker) *SessCond {
	sc := &SessCond{}
	sc.sct = sct
	sc.lock = lock
	sc.conds = make(map[sessp.Tsession][]*cond)
	sc.wakingConds = make(map[sessp.Tsession]map[*cond]bool)
	return sc
}

func (sc *SessCond) alloc(sessid sessp.Tsession) *cond {
	if _, ok := sc.conds[sessid]; !ok {
		sc.conds[sessid] = []*cond{}
	}
	c := &cond{c: sync.NewCond(sc.lock), threadmgr: sc.sct.St.SessThread(sessid)}
	sc.conds[sessid] = append(sc.conds[sessid], c)
	return c
}

// Caller should hold sc lock and will receive it back on return. Wait releases
// sess lock, so that other threads on the session can run. sc.lock ensures
// atomicity of releasing sc lock and going to sleep.
func (sc *SessCond) Wait(sessid sessp.Tsession) *serr.Err {
	c := sc.alloc(sessid)

	if c.threadmgr != nil {
		c.threadmgr.Sleep(c.c)
	} else {
		c.c.Wait()
	}

	sc.removeWakingCond(sessid, c)

	closed := c.isClosed

	if closed {
		db.DPrintf(db.SESSCOND, "wait sess closed %v\n", sessid)
		return serr.MkErr(serr.TErrClosed, fmt.Sprintf("session %v", sessid))
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
			if c.threadmgr != nil {
				c.threadmgr.Wake(c.c)
			} else {
				c.c.Signal()
			}
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Caller should hold sc lock.
func (sc *SessCond) Broadcast() {
	for sid, condlist := range sc.conds {
		for _, c := range condlist {
			if c.threadmgr != nil {
				c.threadmgr.Wake(c.c)
			} else {
				c.c.Signal()
			}
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Keep track of conds which are waiting to be woken up, since their session
// may close in the interim.
func (sc *SessCond) addWakingCond(sid sessp.Tsession, c *cond) {
	if _, ok := sc.wakingConds[sid]; !ok {
		sc.wakingConds[sid] = make(map[*cond]bool)
	}
	sc.wakingConds[sid][c] = true
}

func (sc *SessCond) removeWakingCond(sid sessp.Tsession, c *cond) {
	delete(sc.wakingConds[sid], c)
	if len(sc.wakingConds[sid]) == 0 {
		delete(sc.wakingConds, sid)
	}
}

// A session has been closed: wake up threads associated with this
// session. Grab c lock to ensure that wakeup isn't missed while a
// thread is about enter wait (and releasing sess and sc lock).
func (sc *SessCond) closed(sessid sessp.Tsession) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	db.DPrintf(db.SESSCOND, "cond %p: close %v %v\n", sc, sessid, sc.conds)
	if condlist, ok := sc.conds[sessid]; ok {
		db.DPrintf(db.SESSCOND, "%p: sess %v closed\n", sc, sessid)
		for _, c := range condlist {
			if c.threadmgr != nil {
				c.threadmgr.Wake(c.c)
			} else {
				c.c.Signal()
			}
			sc.addWakingCond(sessid, c)
		}
		delete(sc.conds, sessid)
	}
	// Mark all this session's waking conds as closed.
	for c, _ := range sc.wakingConds[sessid] {
		c.isClosed = true
	}
}
