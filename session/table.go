package session

import (
	"fmt"
	"log"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fence"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

type SessionTable struct {
	sync.Mutex
	//	deadlock.Mutex
	mkps       protsrv.MkProtServer
	fssrv      protsrv.FsServer
	sessions   map[np.Tsession]*Session
	seenFences *fence.FenceTable
}

func MakeSessionTable(mkps protsrv.MkProtServer, fssrv protsrv.FsServer, fm *fence.FenceTable) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*Session)
	st.fssrv = fssrv
	st.mkps = mkps
	st.seenFences = fm
	return st
}

func (st *SessionTable) Lookup(sid np.Tsession) (*Session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) Alloc(sid np.Tsession) *Session {
	st.Lock()
	defer st.Unlock()

	if sess, ok := st.sessions[sid]; ok {
		return sess
	}
	sess := makeSession(st.mkps(st.fssrv, sid), sid, st.seenFences)
	st.sessions[sid] = sess
	return sess
}

func (st *SessionTable) Detach(sid np.Tsession) error {
	sess, ok := st.Lookup(sid)
	if !ok {
		return fmt.Errorf("%v: no sess %v", db.GetName(), sid)
	}
	sess.protsrv.Detach()
	return nil
}

func (st *SessionTable) SessLock(sessid np.Tsession) {
	if sess, ok := st.Lookup(sessid); ok {
		sess.Lock()
		sess.cond.Signal()
	} else {
		log.Fatalf("LockSession: no lock for %v\n", sessid)
	}
}

func (st *SessionTable) SessUnlock(sessid np.Tsession) {
	if sess, ok := st.Lookup(sessid); ok {
		sess.Unlock()
	} else {
		log.Fatalf("UnlockSession: no lock for %v\n", sessid)
	}
}
