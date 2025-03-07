package srv

import (
	"time"

	db "sigmaos/debug"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type Fsrvfcall func(*Session, *sessp.FcallMsg) *sessp.FcallMsg

type sessionMgr struct {
	st       *sessionTable
	srvfcall Fsrvfcall
	done     bool
}

func newSessionMgr(st *sessionTable, pfn Fsrvfcall) *sessionMgr {
	sm := &sessionMgr{}
	sm.st = st
	sm.srvfcall = pfn
	go sm.runDetaches()
	return sm
}

// Force a client on the last session to detach for testing purposes
func (sm *sessionMgr) DisconnectClient() {
	c, sess := sm.st.lastClnt()
	if c != sp.NoClntId {
		db.DPrintf(db.CRASH, "DisconnectClient %v %v", c, sess)
		detach := sessp.NewFcallMsg(&sp.Tdetach{ClntId: uint64(c)}, nil, sess.Sid, nil)
		sm.srvfcall(sess, detach)
	}
}

func (sm *sessionMgr) DisconnectAllClients() {
	c, sess := sm.st.lastClnt()
	// get all clients
	for c != sp.NoClntId {
		db.DPrintf(db.CRASH, "DisconnectClient %v %v", c, sess)
		detach := sessp.NewFcallMsg(&sp.Tdetach{ClntId: uint64(c)}, nil, sess.Sid, nil)
		sm.srvfcall(sess, detach)
		c, sess = sm.st.lastClnt()
	}
}

// Close last the conn associated with last sess for testing purposes
func (sm *sessionMgr) CloseConn() {
	sess := sm.st.lastSession()
	if sess != nil {
		db.DPrintf(db.CRASH, "%v: CloseConn", sess.Sid)
		sess.CloseConn()
	}
}

// Find timed-out sessions.
func (sm *sessionMgr) getTimedOutSessions() []*Session {
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
func (sm *sessionMgr) runDetaches() {
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

func (sm *sessionMgr) Done() bool {
	sm.st.mu.RLock()
	defer sm.st.mu.RUnlock()

	return sm.done
}

func (sm *sessionMgr) Stop() {
	sm.st.mu.Lock()
	defer sm.st.mu.Unlock()

	sm.done = true
}
