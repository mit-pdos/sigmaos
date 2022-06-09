package sessstatesrv

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/threadmgr"
)

type SessionTable struct {
	sync.Mutex
	c *sync.Cond
	//	deadlock.Mutex
	tm       *threadmgr.ThreadMgrTable
	mkps     np.MkProtServer
	fssrv    np.SessServer
	sessions map[np.Tsession]*Session
	last     *Session // for tests
}

func MakeSessionTable(mkps np.MkProtServer, fssrv np.SessServer, tm *threadmgr.ThreadMgrTable) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*Session)
	st.fssrv = fssrv
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

func (st *SessionTable) Alloc(sid np.Tsession) *Session {
	st.Lock()
	defer st.Unlock()

	if sess, ok := st.sessions[sid]; ok {
		return sess
	}
	sess := makeSession(st.mkps(st.fssrv, sid), sid, st.tm.AddThread())
	st.sessions[sid] = sess
	st.last = sess
	return sess
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
