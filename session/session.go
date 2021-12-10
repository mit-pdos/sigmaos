package session

import (
	"fmt"
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

type session struct {
	sync.Mutex
	protsrv protsrv.Protsrv
	session np.Tsession
	dlock   *dlock.Dlock
}

func makeSession(sess np.Tsession, protsrv protsrv.Protsrv) *session {
	s := &session{}
	s.session = sess
	s.protsrv = protsrv
	return s
}

type SessionTable struct {
	sync.Mutex
	mkps     protsrv.MkProtServer
	fssrv    protsrv.FsServer
	sessions map[np.Tsession]*session
}

func MakeSessionTable(mkps protsrv.MkProtServer, fssrv protsrv.FsServer) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*session)
	st.fssrv = fssrv
	st.mkps = mkps
	return st
}

func (st *SessionTable) lookup(id np.Tsession) (*session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[id]
	return sess, ok
}

func (st *SessionTable) LookupInsert(sess np.Tsession) *session {
	st.Lock()
	defer st.Unlock()

	if s, ok := st.sessions[sess]; ok {
		return s
	}
	s := makeSession(sess, st.mkps(st.fssrv))
	st.sessions[sess] = s
	return s
}

// Detach each session
func (st *SessionTable) Detach() {
	st.Lock()
	defer st.Unlock()

	for s, sess := range st.sessions {
		sess.protsrv.Detach(s)
	}
}

func (st *SessionTable) RegisterLock(id np.Tsession, fn []string, qid np.Tqid) error {
	sess, ok := st.lookup(id)
	if !ok {
		return fmt.Errorf("%v: no sess %v", db.GetName(), id)
	}

	sess.Lock()
	defer sess.Unlock()

	if sess.dlock != nil {
		return fmt.Errorf("%v: lock present already %v", db.GetName(), id)
	}

	sess.dlock = dlock.MakeDlock(fn, qid)
	return nil
}

func (st *SessionTable) DeregisterLock(id np.Tsession, fn []string) error {
	sess, ok := st.lookup(id)
	if !ok {
		return fmt.Errorf("%v: Unlock no sess %v", db.GetName(), id)
	}

	sess.Lock()
	defer sess.Unlock()

	if sess.dlock == nil {
		return fmt.Errorf("%v: Unlock no lock %v", db.GetName(), id)
	}

	if !np.IsPathEq(sess.dlock.Fn, fn) {
		return fmt.Errorf("%v: Unlock lock is for %v not %v", db.GetName(), sess.dlock.Fn, fn)
	}

	sess.dlock = nil
	return nil
}

func (st *SessionTable) LockName(id np.Tsession) ([]string, error) {
	sess, ok := st.lookup(id)
	if !ok {
		return nil, fmt.Errorf("%v: LockName no sess %v", db.GetName(), id)
	}

	sess.Lock()
	defer sess.Unlock()
	if sess.dlock == nil {
		return nil, nil
	}
	return sess.dlock.Fn, nil
}

func (st *SessionTable) CheckLock(id np.Tsession, fn []string, qid np.Tqid) error {
	sess, ok := st.lookup(id)
	if !ok {
		return fmt.Errorf("%v: CheckLock no sess %v", db.GetName(), id)
	}

	sess.Lock()
	defer sess.Unlock()

	if sess.dlock == nil {
		return fmt.Errorf("%v: CheckLock no lock %v", db.GetName(), id)
	}

	if !np.IsPathEq(sess.dlock.Fn, fn) {
		return fmt.Errorf("%v: CheckLock lock is for %v not %v", db.GetName(), sess.dlock.Fn, fn)
	}
	return sess.dlock.Check(qid)
}
