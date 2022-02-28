package session

import (
	"encoding/json"
	"log"

	np "ulambda/ninep"
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
