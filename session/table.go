package session

import (
	"log"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	np "ulambda/ninep"
	"ulambda/protsrv"
	"ulambda/threadmgr"
)

type SessionTable struct {
	sync.Mutex
	//	deadlock.Mutex
	tm       *threadmgr.ThreadMgrTable
	mkps     protsrv.MkProtServer
	fssrv    protsrv.FsServer
	sessions map[np.Tsession]*Session
}

func MakeSessionTable(mkps protsrv.MkProtServer, fssrv protsrv.FsServer, tm *threadmgr.ThreadMgrTable) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*Session)
	st.fssrv = fssrv
	st.mkps = mkps
	st.tm = tm
	return st
}

func (st *SessionTable) Lookup(sid np.Tsession) (*Session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) Alloc(sid np.Tsession, replies chan *np.Fcall) *Session {
	st.Lock()
	defer st.Unlock()

	if sess, ok := st.sessions[sid]; ok {
		sess.maybeSetRepliesC(replies)
		return sess
	}
	sess := makeSession(st.mkps(st.fssrv, sid), sid, replies, st.tm.AddThread())
	st.sessions[sid] = sess
	return sess
}

func (st *SessionTable) SessThread(sid np.Tsession) *threadmgr.ThreadMgr {
	if sess, ok := st.Lookup(sid); ok {
		return sess.threadmgr
	} else {
		log.Fatalf("FATAL SessThread: no thread for %v\n", sid)
	}
	return nil
}

func (st *SessionTable) KillSessThread(sid np.Tsession) {
	t := st.SessThread(sid)
	st.Lock()
	defer st.Unlock()
	st.tm.RemoveThread(t)
}
