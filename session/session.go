package session

import (
	"fmt"
	// "log"
	"sync"

	db "ulambda/debug"
	"ulambda/dlock"
	np "ulambda/ninep"
)

type Session struct {
	mu    sync.Mutex
	dlock *dlock.Dlock
}

type SessionTable struct {
	mu       sync.Mutex
	sessions map[np.Tsession]*Session
}

func MakeSessionTable() *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*Session)
	return st
}

func (st *SessionTable) RegisterSession(id np.Tsession) {
	db.DLPrintf("SETAB", "Register session %v", id)

	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.sessions[id]; !ok {
		new := &Session{}
		st.sessions[id] = new
	}
}

func (st *SessionTable) DeleteSession(id np.Tsession) {
	db.DLPrintf("SETAB", "Remove session %v", id)

	st.mu.Lock()
	defer st.mu.Unlock()

	// If the session exists...
	if _, ok := st.sessions[id]; ok {
		delete(st.sessions, id)
	}
}

func (st *SessionTable) Lookup(id np.Tsession) (*Session, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	sess, ok := st.sessions[id]
	return sess, ok
}

func (st *SessionTable) Lock(id np.Tsession, fn []string, qid np.Tqid) error {
	sess, ok := st.Lookup(id)
	if !ok {
		return fmt.Errorf("%v: no sess %v", db.GetName(), id)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.dlock != nil {
		return fmt.Errorf("%v: lock present already %v", db.GetName(), id)
	}

	sess.dlock = dlock.MakeDlock(fn, qid)
	return nil
}

func (st *SessionTable) Unlock(id np.Tsession, fn []string) error {
	sess, ok := st.Lookup(id)
	if !ok {
		return fmt.Errorf("%v: Unlock no sess %v", db.GetName(), id)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

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
	sess, ok := st.Lookup(id)
	if !ok {
		return nil, fmt.Errorf("%v: LockName no sess %v", db.GetName(), id)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.dlock == nil {
		return nil, nil
	}
	return sess.dlock.Fn, nil
}

func (st *SessionTable) CheckLock(id np.Tsession, fn []string, qid np.Tqid) error {
	sess, ok := st.Lookup(id)
	if !ok {
		return fmt.Errorf("%v: CheckLock no sess %v", db.GetName(), id)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.dlock == nil {
		return fmt.Errorf("%v: CheckLock no lock %v", db.GetName(), id)
	}

	if !np.IsPathEq(sess.dlock.Fn, fn) {
		return fmt.Errorf("%v: CheckLock lock is for %v not %v", db.GetName(), sess.dlock.Fn, fn)
	}
	return sess.dlock.Check(qid)
}
