// package clntcond wraps cond vars so that if a client terminates, it
// can wakeup threads that are associated with that client.  Each cond
// var is represented as several cond vars, one per goroutine using
// it.
package clntcond

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

// Clnt cond has one cond per client.  The lock is, for example, a
// pipe lock or watch lock, which ClntCond releases in Wait() and
// re-acquires before returning out of Wait().
type ClntCond struct {
	lock        sync.Locker
	sct         *ClntCondTable
	nref        int // under sct lock
	conds       map[sp.TclntId][]*cond
	wakingConds map[sp.TclntId]map[*cond]bool // Conds pending wakeup (which also may need to be alerted that the clnt has detached)
}

func newClntCond(sct *ClntCondTable, lock sync.Locker) *ClntCond {
	sc := &ClntCond{}
	sc.sct = sct
	sc.lock = lock
	sc.conds = make(map[sp.TclntId][]*cond)
	sc.wakingConds = make(map[sp.TclntId]map[*cond]bool)
	return sc
}

func (sc *ClntCond) String() string {
	s := fmt.Sprintf("ClntCond: cond: [")
	for cid, sl := range sc.conds {
		s += fmt.Sprintf("%v %v ", cid, sl)
	}
	s += fmt.Sprintf("], [")
	for cid, m := range sc.wakingConds {
		s += fmt.Sprintf("%v %p ", cid, m)
	}
	s += fmt.Sprintf("]")
	return s
}

func (sc *ClntCond) alloc(cid sp.TclntId) *cond {
	if _, ok := sc.conds[cid]; !ok {
		sc.conds[cid] = []*cond{}
	}
	c := &cond{c: sync.NewCond(sc.lock)}
	db.DPrintf(db.CLNTCOND, "alloc sc %v %v c %v\n", sc, cid, c)
	sc.conds[cid] = append(sc.conds[cid], c)
	return c
}

// Caller should hold sc lock and will receive it back on return. Wait
// releases sess lock, so that other threads associated with client
// can run. sc.lock ensures atomicity of releasing sc lock and going
// to sleep.
func (sc *ClntCond) Wait(cid sp.TclntId) *serr.Err {
	c := sc.alloc(cid)

	db.DPrintf(db.CLNTCOND, "Wait %v c %v\n", cid, c)

	c.c.Wait()

	sc.removeWakingCond(cid, c)

	closed := c.isClosed

	db.DPrintf(db.CLNTCOND, "Wait return %v\n", c)

	if closed {
		db.DPrintf(db.CLNTCOND, "wait clnt closed %v\n", cid)
		return serr.NewErr(serr.TErrClosed, fmt.Sprintf("client %v", cid))
	}
	return nil
}

// Caller should hold sc lock and the sleeper should have gone to
// sleep while holding sc.lock and release it in inside of sleep, so
// it shouldn't be running anymore.
func (sc *ClntCond) Signal() {
	for sid, condlist := range sc.conds {
		for _, c := range condlist {
			db.DPrintf(db.CLNTCOND, "Signal %v c %v\n", sid, c)
			c.c.Signal()
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Caller should hold sc lock.
func (sc *ClntCond) Broadcast() {
	for sid, condlist := range sc.conds {
		for _, c := range condlist {
			c.c.Signal()
			sc.addWakingCond(sid, c)
		}
		delete(sc.conds, sid)
	}
}

// Keep track of conds which are waiting to be woken up, since a client
// may detach in the interim.
func (sc *ClntCond) addWakingCond(sid sp.TclntId, c *cond) {
	if _, ok := sc.wakingConds[sid]; !ok {
		sc.wakingConds[sid] = make(map[*cond]bool)
	}
	sc.wakingConds[sid][c] = true
}

func (sc *ClntCond) removeWakingCond(sid sp.TclntId, c *cond) {
	delete(sc.wakingConds[sid], c)
	if len(sc.wakingConds[sid]) == 0 {
		delete(sc.wakingConds, sid)
	}
}

// A client has detached: wake up threads associated with this
// client. Grab c lock to ensure that wakeup isn't missed while a
// thread is about enter wait (and releasing sess and sc lock).
func (sc *ClntCond) closed(cid sp.TclntId) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	db.DPrintf(db.CLNTCOND, "cond %p: close %v %v\n", sc, cid, sc.conds)
	if condlist, ok := sc.conds[cid]; ok {
		db.DPrintf(db.CLNTCOND, "%p: sess %v closed\n", sc, cid)
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
