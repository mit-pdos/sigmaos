package session

import (
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type SessionMgr struct {
	st       *SessionTable
	srvfcall np.Fsrvfcall
	done     bool
}

func MakeSessionMgr(st *SessionTable, pfn np.Fsrvfcall) *SessionMgr {
	sm := &SessionMgr{}
	sm.st = st
	sm.srvfcall = pfn
	go sm.run()
	return sm
}

func (sm *SessionMgr) FindASession() *Session {
	sm.st.Lock()
	defer sm.st.Unlock()
	for _, sess := range sm.st.sessions {
		return sess
	}
	return nil
}

// Force one session to timeout
func (sm *SessionMgr) TimeoutSession() {
	sess := sm.FindASession()
	if sess != nil {
		sess.timeout()
	}
}

func (sm *SessionMgr) CloseConn() {
	sess := sm.FindASession()
	if sess != nil {
		sess.CloseConn()
	}
}

// Find timed-out sessions.
func (sm *SessionMgr) getTimedOutSessions() []*Session {
	// Lock the session table.
	sm.st.Lock()
	defer sm.st.Unlock()
	sess := make([]*Session, 0, len(sm.st.sessions))
	for sid, s := range sm.st.sessions {
		// Find timed-out sessions which haven't been closed yet.
		if s.timedOut() && !s.IsClosed() {
			db.DPrintf("SESSION_ERR", "Sess %v timed out; close it", sid)
			sess = append(sess, s)
		}
	}
	return sess
}

// Scan for detachable sessions, and request that they be detahed.
func (sm *SessionMgr) run() {
	for !sm.Done() {
		// Sleep for a bit.
		time.Sleep(np.SESSTIMEOUTMS * time.Millisecond)
		sess := sm.getTimedOutSessions()
		for _, s := range sess {
			detach := np.MakeFcall(np.Tdetach{}, s.Sid, nil, np.NoFence)
			sm.srvfcall(detach)
		}
	}
}

func (sm *SessionMgr) Done() bool {
	return sm.done
}

func (sm *SessionMgr) Stop() {
	sm.done = true
}
