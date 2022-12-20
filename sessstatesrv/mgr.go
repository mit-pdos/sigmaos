package sessstatesrv

import (
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type SessionMgr struct {
	st       *SessionTable
	srvfcall sp.Fsrvfcall
	done     bool
}

func MakeSessionMgr(st *SessionTable, pfn sp.Fsrvfcall) *SessionMgr {
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
func (sm *SessionMgr) getConnectedSessions() []uint64 {
	// Lock the session table.
	sm.st.Lock()
	defer sm.st.Unlock()
	sess := make([]uint64, 0, len(sm.st.sessions))
	for sid, s := range sm.st.sessions {
		// Find timed-out sessions which haven't been closed yet.
		if s.isConnected() {
			db.DPrintf(db.SESS_STATE_SRV, "Sess %v is connected, generating heartbeat.", sid)
			sess = append(sess, uint64(s.Sid))
		}
	}
	return sess
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
		hbs := sp.MakeFcallMsg(sp.MkTheartbeat(sess), nil, 0, 0, nil, nil, sp.MakeFenceNull())
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
			detach := sp.MakeFcallMsg(&sp.Tdetach{}, nil, s.ClientId, s.Sid, nil, nil, sp.MakeFenceNull())
			sm.srvfcall(detach)
		}
	}
}

func (sm *SessionMgr) Done() bool {
	sm.st.Lock()
	defer sm.st.Unlock()

	return sm.done
}

func (sm *SessionMgr) Stop() {
	sm.st.Lock()
	defer sm.st.Unlock()

	sm.done = true
}
