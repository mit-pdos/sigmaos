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
	dlock   *dlock.Dlock
}

func makeSession(protsrv protsrv.Protsrv) *session {
	sess := &session{}
	sess.protsrv = protsrv
	return sess
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

func (st *SessionTable) lookup(sid np.Tsession) (*session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) LookupInsert(sid np.Tsession) *session {
	st.Lock()
	defer st.Unlock()

	if sess, ok := st.sessions[sid]; ok {
		return sess
	}
	sess := makeSession(st.mkps(st.fssrv))
	st.sessions[sid] = sess
	return sess
}

func (st *SessionTable) Detach(sid np.Tsession) error {
	sess, ok := st.lookup(sid)
	if !ok {
		return fmt.Errorf("%v: no sess %v", db.GetName(), sid)
	}

	st.Lock()
	defer st.Unlock()
	sess.protsrv.Detach(sid)
	return nil
}

func (st *SessionTable) RegisterLock(sid np.Tsession, fn []string, qid np.Tqid) error {
	sess, ok := st.lookup(sid)
	if !ok {
		return fmt.Errorf("%v: no sess %v", db.GetName(), sid)
	}

	sess.Lock()
	defer sess.Unlock()

	if sess.dlock != nil {
		return fmt.Errorf("%v: lock present already %v", db.GetName(), sid)
	}

	sess.dlock = dlock.MakeDlock(fn, qid)
	return nil
}

func (st *SessionTable) DeregisterLock(sid np.Tsession, fn []string) error {
	sess, ok := st.lookup(sid)
	if !ok {
		return fmt.Errorf("%v: Unlock no sess %v", db.GetName(), sid)
	}

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

func (st *SessionTable) LockName(sid np.Tsession) ([]string, error) {
	sess, ok := st.lookup(sid)
	if !ok {
		return nil, fmt.Errorf("%v: LockName no sess %v", db.GetName(), sid)
	}

	sess.Lock()
	defer sess.Unlock()
	if sess.dlock == nil {
		return nil, nil
	}
	return sess.dlock.Fn, nil
}

func (st *SessionTable) CheckLock(sid np.Tsession, fn []string, qid np.Tqid) error {
	sess, ok := st.lookup(sid)
	if !ok {
		return fmt.Errorf("%v: CheckLock no sess %v", db.GetName(), sid)
	}

	sess.Lock()
	defer sess.Unlock()

	if sess.dlock == nil {
		return fmt.Errorf("%v: CheckLock no lock %v", db.GetName(), sid)
	}

	if !np.IsPathEq(sess.dlock.Fn, fn) {
		return fmt.Errorf("%v: CheckLock lock is for %v not %v", db.GetName(), sess.dlock.Fn, fn)
	}
	return sess.dlock.Check(qid)
}
