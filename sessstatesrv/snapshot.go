package sessstatesrv

import (
	"encoding/json"

	db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/threadmgr"
)

func (st *SessionTable) Snapshot() []byte {
	sessions := make(map[sessp.Tsession][]byte)
	for sid, sess := range st.sessions {
		sessions[sid] = sess.Snapshot()
	}
	b, err := json.Marshal(sessions)
	if err != nil {
		db.DFatalf("Error snapshot encoding session table: %v", err)
	}
	return b
}

func RestoreTable(oldSt *SessionTable, mkps sp.MkProtServer, rps sp.RestoreProtServer, sesssrv sp.SessServer, tm *threadmgr.ThreadMgrTable, b []byte) *SessionTable {
	sessions := make(map[sessp.Tsession][]byte)
	err := json.Unmarshal(b, &sessions)
	if err != nil {
		db.DFatalf("error unmarshal session table in restore: %v", err)
	}
	st := MakeSessionTable(mkps, sesssrv, tm)
	for sid, b := range sessions {
		st.sessions[sid] = RestoreSession(sid, sesssrv, rps, tm, b)
		// Set the replies channel if this sesison already exists at this replica
		if oldSess, ok := oldSt.Lookup(sid); ok {
			st.sessions[sid].SetConn(oldSess.GetConn())
		}
	}
	return st
}

type SessionSnapshot struct {
	ProtsrvSnap []byte
	ClientId    sessp.Tclient
	closed      bool
}

func MakeSessionSnapshot() *SessionSnapshot {
	return &SessionSnapshot{}
}

func (sess *Session) Snapshot() []byte {
	ss := MakeSessionSnapshot()
	ss.ProtsrvSnap = sess.protsrv.Snapshot()
	ss.ClientId = sess.ClientId
	ss.closed = sess.closed
	b, err := json.Marshal(ss)
	if err != nil {
		db.DFatalf("Error snapshot encoding session: %v", err)
	}
	return b
}

func RestoreSession(sid sessp.Tsession, sesssrv sp.SessServer, rps sp.RestoreProtServer, tmt *threadmgr.ThreadMgrTable, b []byte) *Session {
	ss := MakeSessionSnapshot()
	err := json.Unmarshal(b, ss)
	if err != nil {
		db.DFatalf("error unmarshal session in restore: %v", err)
	}
	fos := rps(sesssrv, ss.ProtsrvSnap)
	// TODO: add session manager
	sess := makeSession(fos, ss.ClientId, sid, tmt.AddThread())
	sess.closed = ss.closed
	return sess
}
