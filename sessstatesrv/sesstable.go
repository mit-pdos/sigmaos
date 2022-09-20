package sessstatesrv

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/threadmgr"
)

type SessionTable struct {
	sync.Mutex
	c *sync.Cond
	//	deadlock.Mutex
	tm       *threadmgr.ThreadMgrTable
	mkps     np.MkProtServer
	sesssrv  np.SessServer
	sessions map[np.Tsession]*Session
	last     *Session // for tests
}

func MakeSessionTable(mkps np.MkProtServer, sesssrv np.SessServer, tm *threadmgr.ThreadMgrTable) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*Session)
	st.sesssrv = sesssrv
	st.mkps = mkps
	st.tm = tm
	st.c = sync.NewCond(&st.Mutex)
	return st
}

func (st *SessionTable) Lookup(sid np.Tsession) (*Session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) Alloc(cid np.Tclient, sid np.Tsession) *Session {
	st.Lock()
	defer st.Unlock()

	return st.allocL(cid, sid)
}

func (st *SessionTable) allocL(cid np.Tclient, sid np.Tsession) *Session {
	if sess, ok := st.sessions[sid]; ok {
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

func (st *SessionTable) SessThread(sid np.Tsession) *threadmgr.ThreadMgr {
	if sess, ok := st.Lookup(sid); ok {
		return sess.threadmgr
	} else {
		db.DFatalf("SessThread: no thread for %v\n", sid)
	}
	return nil
}

func (st *SessionTable) KillSessThread(sid np.Tsession) {
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

func (st *SessionTable) WaitClosed() {
	st.Lock()
	defer st.Unlock()
	db.DPrintf("SESSION", "Wait for open sess %v\n", len(st.sessions))
	//for len(st.sessions) > 0 {
	//	st.c.Wait()
	//}
}
