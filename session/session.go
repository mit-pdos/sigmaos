package session

import (
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/dlock"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

//
// A session identifies a client across TCP connections.  For each
// session, sigmaos has a protsrv.
//

type Session struct {
	sync.Mutex
	protsrv protsrv.Protsrv
	dlock   *dlock.Dlock
}

func makeSession(protsrv protsrv.Protsrv) *Session {
	sess := &Session{}
	sess.protsrv = protsrv
	return sess
}

func (sess *Session) RegisterLock(sid np.Tsession, fn []string, qid np.Tqid) error {
	sess.Lock()
	defer sess.Unlock()

	if sess.dlock != nil {
		return fmt.Errorf("%v: lock present already %v", db.GetName(), sid)
	}

	log.Printf("%v: registerlock %v %v\n", db.GetName(), sid, fn)

	sess.dlock = dlock.MakeDlock(fn, qid)
	return nil
}

func (sess *Session) DeregisterLock(sid np.Tsession, fn []string) error {
	sess.Lock()
	defer sess.Unlock()

	if sess.dlock == nil {
		return fmt.Errorf("%v: Unlock no lock %v", db.GetName(), sid)
	}

	if !np.IsPathEq(sess.dlock.Fn, fn) {
		return fmt.Errorf("%v: Unlock lock is for %v not %v", db.GetName(), sess.dlock.Fn, fn)
	}

	sess.dlock = nil
	return nil
}

func (sess *Session) CheckLock(fn []string, qid np.Tqid) error {
	sess.Lock()
	defer sess.Unlock()

	if sess.dlock == nil {
		return fmt.Errorf("%v: CheckLock no lock", db.GetName())
	}

	if !np.IsPathEq(sess.dlock.Fn, fn) {
		return fmt.Errorf("%v: CheckLock lock is for %v not %v", db.GetName(), sess.dlock.Fn, fn)
	}
	return sess.dlock.Check(qid)
}

func (sess *Session) LockName() ([]string, error) {
	sess.Lock()
	defer sess.Unlock()
	if sess.dlock == nil {
		return nil, nil
	}
	return sess.dlock.Fn, nil
}
