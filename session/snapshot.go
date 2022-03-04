package session

import (
	"encoding/json"
	"log"

	"ulambda/fences"
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

func RestoreTable(mkps protsrv.MkProtServer, rps protsrv.RestoreProtServer, fssrv protsrv.FsServer, rt *fences.RecentTable, tm *threadmgr.ThreadMgrTable, b []byte) *SessionTable {
	sessions := make(map[np.Tsession][]byte)
	err := json.Unmarshal(b, sessions)
	if err != nil {
		log.Fatalf("FATAL error unmarshal session table in restore: %v", err)
	}
	st := MakeSessionTable(mkps, fssrv, rt, tm)
	for session, b := range sessions {
		st.sessions[session] = RestoreSession(session, rps, rt, tm, b)
	}
	return st
}

type SessionSnapshot struct {
	ProtsrvSnap []byte
	FencesSnap  []byte
}

func MakeSessionSnapshot() *SessionSnapshot {
	return &SessionSnapshot{}
}

func (sess *Session) Snapshot() []byte {
	ss := MakeSessionSnapshot()
	ss.ProtsrvSnap = sess.protsrv.Snapshot()
	ss.FencesSnap = sess.myFences.Snapshot()
	b, err := json.Marshal(ss)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding session: %v", err)
	}
	return b
}

func RestoreSession(sid np.Tsession, rps protsrv.RestoreProtServer, rt *fences.RecentTable, tm *threadmgr.ThreadMgrTable, b []byte) *Session {
	ss := MakeSessionSnapshot()
	err := json.Unmarshal(b, ss)
	if err != nil {
		log.Fatalf("FATAL error unmarshal session in restore: %v", err)
	}
	fos := rps(nil, ss.ProtsrvSnap)
	sess := makeSession(fos, sid, rt, tm.AddThread())
	myFences := fences.RestoreFenceTable(ss.FencesSnap)
	sess.myFences = myFences
	return sess
}
