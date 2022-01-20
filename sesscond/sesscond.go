package sesscond

import (
	"fmt"
	"log"
	"sync"
	// "errors"

	np "ulambda/ninep"
)

//
// sesscond wraps cond vars so that if a session terminates, it can
// wakeup threads that are associated with that session.  Each cond
// var is represented as several cond vars, one per session.
//

type cond struct {
	isClosed bool
	c        *sync.Cond
}

type SessCond struct {
	lock  *sync.Mutex
	conds map[np.Tsession]*cond
}

func makeSessCond(lock *sync.Mutex) *SessCond {
	sc := &SessCond{}
	sc.lock = lock
	sc.conds = make(map[np.Tsession]*cond)
	return sc
}

func (sc *SessCond) closed(sessid np.Tsession) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	if c, ok := sc.conds[sessid]; ok {
		c.isClosed = true
		c.c.Broadcast()
	}
}

func (sc *SessCond) alloc(sessid np.Tsession) *cond {
	if c, ok := sc.conds[sessid]; ok {
		return c
	}
	c := &cond{false, sync.NewCond(sc.lock)}
	sc.conds[sessid] = c
	return c
}

// caller should hold sc lock and get it back on return
func (sc *SessCond) Wait(sessid np.Tsession) error {
	c := sc.alloc(sessid)
	c.c.Wait()
	if c.isClosed {
		log.Printf("wait sess closed\n", sessid)
		return fmt.Errorf("session closed %v", sessid)
	}
	return nil
}

func (sc *SessCond) Signal() {
	for _, c := range sc.conds {
		c.c.Signal()
	}
}

type SessCondTable struct {
	sync.Mutex
	conds map[*SessCond]bool
}

func MakeSessCondTable() *SessCondTable {
	t := &SessCondTable{}
	t.conds = make(map[*SessCond]bool)
	return t
}

func (sct *SessCondTable) MakeSessCond(lock *sync.Mutex) *SessCond {
	sct.Lock()
	defer sct.Unlock()

	sc := makeSessCond(lock)
	sct.conds[sc] = true
	return sc
}

func (sct *SessCondTable) DeleteSess(sessid np.Tsession) {
	sct.Lock()
	defer sct.Unlock()

	// log.Printf("%v: DeleteSess %v\n", db.GetName(), sct.conds)
	for sc, _ := range sct.conds {
		sc.closed(sessid)
	}
}
