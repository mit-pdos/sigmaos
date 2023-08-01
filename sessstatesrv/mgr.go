package sessstatesrv

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type SessionMgr struct {
	st       *SessionTable
	srvfcall sps.Fsrvfcall
	done     bool
}

func MakeSessionMgr(st *SessionTable, pfn sps.Fsrvfcall) *SessionMgr {
	sm := &SessionMgr{}
	sm.st = st
	sm.srvfcall = pfn
	go sm.runHeartbeats()
	go sm.runDetaches()
	return sm
}

// Force the last session to timeout
func (sm *SessionMgr) TimeoutSession() {
	sess := sm.st.LastSession()
	if sess != nil {
		db.DPrintf(db.SESS_STATE_SRV, "Test TimeoutSession %v", sess.Sid)
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

// Find connected sessions.
func (sm *SessionMgr) getConnectedSessions() map[uint64]bool {
	// Lock the session table.
	sm.st.mu.RLock()
	defer sm.st.mu.RUnlock()
	sess := make(map[uint64]bool, len(sm.st.sessions))
	for sid, s := range sm.st.sessions {
		// Find timed-out sessions which haven't been closed yet.
		if s.isConnected() {
			db.DPrintf(db.SESS_STATE_SRV, "Sess %v is connected, generating heartbeat.", sid)
			sess[uint64(s.Sid)] = true
		}
	}
	return sess
}

// Find timed-out sessions.
func (sm *SessionMgr) getTimedOutSessions() []*Session {
	// Lock the session table.
	sm.st.mu.RLock()
	defer sm.st.mu.RUnlock()
	sess := make([]*Session, 0, len(sm.st.sessions))
	for sid, s := range sm.st.sessions {
		// Find timed-out sessions which haven't been closed yet.
		if timedout, lhb := s.timedOut(); timedout && !s.IsClosed() {
			db.DPrintf(db.SESS_STATE_SRV_ERR, "Sess %v timed out, last heartbeat: %v", sid, lhb)
			sess = append(sess, s)
		}
	}
	return sess
}

// Scan for live/connected sessions, and send heartbeats on their behalf.
func (sm *SessionMgr) runHeartbeats() {
	sessHeartbeatT := time.NewTicker(sp.Conf.Session.HEARTBEAT_INTERVAL)
	for !sm.Done() {
		<-sessHeartbeatT.C
		sess := sm.getConnectedSessions()
		hbs := sessp.MakeFcallMsg(sp.MkTheartbeat(sess), nil, 0, 0, nil, sessp.Tinterval{}, sessp.NullFence())
		sm.srvfcall(hbs)
	}
}

// Scan for detachable sessions, and request that they be detached.
func (sm *SessionMgr) runDetaches() {
	sessTimeoutT := time.NewTicker(sp.Conf.Session.TIMEOUT)

	for !sm.Done() {
		<-sessTimeoutT.C
		sess := sm.getTimedOutSessions()
		for _, s := range sess {
			detach := sessp.MakeFcallMsg(&sp.Tdetach{}, nil, s.ClientId, s.Sid, nil, sessp.Tinterval{}, sessp.NullFence())
			sm.srvfcall(detach)
		}
	}
}

func (sm *SessionMgr) Done() bool {
	sm.st.mu.RLock()
	defer sm.st.mu.RUnlock()

	return sm.done
}

func (sm *SessionMgr) Stop() {
	sm.st.mu.Lock()
	defer sm.st.mu.Unlock()

	sm.done = true
}
