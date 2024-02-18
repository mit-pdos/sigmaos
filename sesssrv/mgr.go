package sesssrv

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Fsrvfcall func(*Session, *sessp.FcallMsg) *sessp.FcallMsg

type SessionMgr struct {
	st       *SessionTable
	srvfcall Fsrvfcall
	done     bool
}

func NewSessionMgr(st *SessionTable, pfn Fsrvfcall) *SessionMgr {
	sm := &SessionMgr{}
	sm.st = st
	sm.srvfcall = pfn
	go sm.runDetaches()
	return sm
}

// Force a client on the last session to detach for testing purposes
func (sm *SessionMgr) DisconnectClient() {
	c, sess := sm.st.lastClnt()
	if c != sp.NoClntId {
		db.DPrintf(db.CRASH, "DisconnectClient %v %v", c, sess)
		detach := sessp.NewFcallMsg(&sp.Tdetach{ClntId: uint64(c)}, nil, sess.Sid, nil)
		sm.srvfcall(sess, detach)
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

// Find timed-out sessions.
func (sm *SessionMgr) getTimedOutSessions() []*Session {
	// Lock the session table.
	sm.st.mu.RLock()
	defer sm.st.mu.RUnlock()
	sess := make([]*Session, 0, len(sm.st.sessions))
	for sid, s := range sm.st.sessions {
		if s.IsConnected() {
			db.DPrintf(db.SESSSRV, "Sess %v is connected", sid)
			s.lastHeartbeat = time.Now()
			continue
		}
		// Find timed-out sessions which haven't been closed yet.
		if timedout, lhb := s.timedOut(); timedout && !s.IsClosed() {
			db.DPrintf(db.SESSSRV, "Sess %v timed out, last heartbeat: %v", sid, lhb)
			sess = append(sess, s)
		}
	}
	return sess
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
				db.DPrintf(db.ALWAYS, "Sess %v Clnt %v timed out", s.Sid, c)
				detach := sessp.NewFcallMsg(&sp.Tdetach{ClntId: uint64(c)}, nil, s.Sid, nil)
				sm.srvfcall(s, detach)
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
