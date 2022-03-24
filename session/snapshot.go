package session

import (
	"encoding/json"
	"log"

	np "ulambda/ninep"
	"ulambda/protsrv"
	"ulambda/threadmgr"
)

func (st *SessionTable) Snapshot() []byte {
	sessions := make(map[np.Tsession][]byte)
	for sid, sess := range st.sessions {
		sessions[sid] = sess.Snapshot()
	}
	b, err := json.Marshal(sessions)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding session table: %v", err)
	}
	return b
}

func RestoreTable(mkps protsrv.MkProtServer, rps protsrv.RestoreProtServer, fssrv protsrv.FsServer, tm *threadmgr.ThreadMgrTable, b []byte) *SessionTable {
	sessions := make(map[np.Tsession][]byte)
	err := json.Unmarshal(b, &sessions)
	if err != nil {
		log.Fatalf("FATAL error unmarshal session table in restore: %v", err)
	}
	st := MakeSessionTable(mkps, fssrv, tm)
	for session, b := range sessions {
		st.sessions[session] = RestoreSession(session, fssrv, rps, tm, b)
	}
	return st
}

type SessionSnapshot struct {
	ProtsrvSnap []byte
	closed      bool
}

func MakeSessionSnapshot() *SessionSnapshot {
	return &SessionSnapshot{}
}

func (sess *Session) Snapshot() []byte {
	ss := MakeSessionSnapshot()
	ss.ProtsrvSnap = sess.protsrv.Snapshot()
	ss.closed = sess.closed
	b, err := json.Marshal(ss)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding session: %v", err)
	}
	return b
}

func RestoreSession(sid np.Tsession, fssrv protsrv.FsServer, rps protsrv.RestoreProtServer, tmt *threadmgr.ThreadMgrTable, b []byte) *Session {
	ss := MakeSessionSnapshot()
	err := json.Unmarshal(b, ss)
	if err != nil {
		log.Fatalf("FATAL error unmarshal session in restore: %v", err)
	}
	fos := rps(fssrv, ss.ProtsrvSnap)
	// TODO: add session manager
	sess := makeSession(fos, sid, nil, tmt.AddThread())
	sess.closed = ss.closed
	return sess
}
