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

func NewSessionMgr(st *SessionTable, pfn sps.Fsrvfcall) *SessionMgr {
	sm := &SessionMgr{}
	sm.st = st
	sm.srvfcall = pfn
	go sm.runHeartbeats()
	go sm.runDetaches()
	return sm
}

// Force a client on the last session to detach for testing purposes
func (sm *SessionMgr) DisconnectClient() {
	c, sid := sm.st.lastClnt()
	if c != sp.NoClntId {
		db.DPrintf(db.CRASH, "DisconnectClient %v %v", c, sid)
		detach := sessp.NewFcallMsg(&sp.Tdetach{ClntId: uint64(c)}, nil, sid, nil)
		sm.srvfcall(detach)
	}
}

// Close last the conn associated with last sess for testing purposes
func (sm *SessionMgr) CloseConn() {
	sess := sm.st.lastSession()
	if sess != nil {
		db.DPrintf(db.CRASH, "%v: CloseConn", sess.Sid)
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
		hbs := sessp.NewFcallMsg(sp.NewTheartbeat(sess), nil, 0, nil)
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
			clnts := s.getClnts()
			for _, c := range clnts {
				db.DPrintf(db.ALWAYS, "Session %v timed out", s.Sid)
				detach := sessp.NewFcallMsg(&sp.Tdetach{ClntId: uint64(c)}, nil, s.Sid, nil)
				sm.srvfcall(detach)
			}
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
