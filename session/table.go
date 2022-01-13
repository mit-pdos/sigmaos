package session

import (
	"fmt"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

type SessionTable struct {
	sync.Mutex
	mkps     protsrv.MkProtServer
	fssrv    protsrv.FsServer
	sessions map[np.Tsession]*Session
}

func MakeSessionTable(mkps protsrv.MkProtServer, fssrv protsrv.FsServer) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*Session)
	st.fssrv = fssrv
	st.mkps = mkps
	return st
}

func (st *SessionTable) Lookup(sid np.Tsession) (*Session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) LookupInsert(sid np.Tsession) *Session {
	st.Lock()
	defer st.Unlock()

	if sess, ok := st.sessions[sid]; ok {
		return sess
	}
	sess := makeSession(st.mkps(st.fssrv, sid))
	st.sessions[sid] = sess
	return sess
}

func (st *SessionTable) Detach(sid np.Tsession) error {
	sess, ok := st.Lookup(sid)
	if !ok {
		return fmt.Errorf("%v: no sess %v", db.GetName(), sid)
	}

	st.Lock()
	defer st.Unlock()
	sess.protsrv.Detach()
	return nil
}
