package sessstatesrv

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

// Force the last session to timeout
func (sm *SessionMgr) TimeoutSession() {
	sess := sm.st.LastSession()
	if sess != nil {
		sess.timeout()
	}
}

// Close last the conn associated with last sess
func (sm *SessionMgr) CloseConn() {
	sess := sm.st.LastSession()
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
		if timedout, lhb := s.timedOut(); timedout && !s.IsClosed() {
			db.DPrintf("SESSION_ERR", "Sess %v timed out, last heartbeat: %v", sid, lhb)
			sess = append(sess, s)
		}
	}
	return sess
}

// Scan for detachable sessions, and request that they be detahed.
func (sm *SessionMgr) run() {
	for !sm.Done() {
		// Sleep for a bit.
		time.Sleep(np.Conf.Session.TIMEOUT_MS)
		sess := sm.getTimedOutSessions()
		for _, s := range sess {
			detach := np.MakeFcall(np.Tdetach{}, s.Sid, nil, nil, np.NoFence)
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
