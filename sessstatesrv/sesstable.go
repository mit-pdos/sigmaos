package sessstatesrv

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fcall"
	np "sigmaos/sigmap"
	"sigmaos/threadmgr"
)

type SessionTable struct {
	sync.Mutex
	c *sync.Cond
	//	deadlock.Mutex
	tm       *threadmgr.ThreadMgrTable
	mkps     np.MkProtServer
	sesssrv  np.SessServer
	sessions map[fcall.Tsession]*Session
	last     *Session // for tests
}

func MakeSessionTable(mkps np.MkProtServer, sesssrv np.SessServer, tm *threadmgr.ThreadMgrTable) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[fcall.Tsession]*Session)
	st.sesssrv = sesssrv
	st.mkps = mkps
	st.tm = tm
	st.c = sync.NewCond(&st.Mutex)
	return st
}

func (st *SessionTable) QueueLen() int {
	st.Lock()
	defer st.Unlock()
	len := 0
	for _, s := range st.sessions {
		len += s.QueueLen()
	}
	return len
}

func (st *SessionTable) Lookup(sid fcall.Tsession) (*Session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) Alloc(cid fcall.Tclient, sid fcall.Tsession) *Session {
	st.Lock()
	defer st.Unlock()

	return st.allocL(cid, sid)
}

func (st *SessionTable) allocL(cid fcall.Tclient, sid fcall.Tsession) *Session {
	if sess, ok := st.sessions[sid]; ok {
		sess.Lock()
		defer sess.Unlock()
		if sess.ClientId == 0 {
			sess.ClientId = cid
		}
		return sess
	}
	sess := makeSession(st.mkps(st.sesssrv, sid), cid, sid, st.tm.AddThread())
	st.sessions[sid] = sess
	st.last = sess
	return sess
}

func (st *SessionTable) ProcessHeartbeats(hbs *np.Theartbeat) {
	st.Lock()
	defer st.Unlock()

	for _, sid := range hbs.Sids {
		sess := st.allocL(0, sid)
		sess.Lock()
		if !sess.closed {
			sess.heartbeatL(hbs)
		}
		sess.Unlock()
	}
}

func (st *SessionTable) SessThread(sid fcall.Tsession) *threadmgr.ThreadMgr {
	if sess, ok := st.Lookup(sid); ok {
		return sess.threadmgr
	} else {
		db.DFatalf("SessThread: no thread for %v\n", sid)
	}
	return nil
}

func (st *SessionTable) KillSessThread(sid fcall.Tsession) {
	t := st.SessThread(sid)
	st.Lock()
	defer st.Unlock()
	st.tm.RemoveThread(t)
}

func (st *SessionTable) LastSession() *Session {
	st.Lock()
	defer st.Unlock()
	if st.last != nil {
		return st.last
	}
	return nil
}
