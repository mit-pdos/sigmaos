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

func (sess *Session) Snapshot() []byte {
	return sess.protsrv.Snapshot()
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

func RestoreSession(sid np.Tsession, rps protsrv.RestoreProtServer, rt *fences.RecentTable, tm *threadmgr.ThreadMgrTable, b []byte) *Session {
	fos := rps(b)
	sess := makeSession(fos, sid, rt, tm.AddThread())
	err := json.Unmarshal(b, sess)
	if err != nil {
		log.Fatalf("FATAL error unmarshal session  in restore: %v", err)
	}
	return sess
}
