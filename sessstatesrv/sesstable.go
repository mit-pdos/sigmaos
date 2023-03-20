package sessstatesrv

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
	"sigmaos/threadmgr"
)

type SessionTable struct {
	sync.Mutex
	c *sync.Cond
	//	deadlock.Mutex
	tm       *threadmgr.ThreadMgrTable
	mkps     sps.MkProtServer
	sesssrv  sps.SessServer
	sessions map[sessp.Tsession]*Session
	last     *Session // for tests
}

func MakeSessionTable(mkps sps.MkProtServer, sesssrv sps.SessServer, tm *threadmgr.ThreadMgrTable) *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[sessp.Tsession]*Session)
	st.sesssrv = sesssrv
	st.mkps = mkps
	st.tm = tm
	st.c = sync.NewCond(&st.Mutex)
	return st
}

func (st *SessionTable) QueueLen() int64 {
	st.Lock()
	defer st.Unlock()
	len := int64(0)
	for _, s := range st.sessions {
		len += s.QueueLen()
	}
	return len
}

func (st *SessionTable) Lookup(sid sessp.Tsession) (*Session, bool) {
	st.Lock()
	defer st.Unlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) Alloc(cid sessp.Tclient, sid sessp.Tsession) *Session {
	st.Lock()
	defer st.Unlock()

	return st.allocL(cid, sid)
}

func (st *SessionTable) allocL(cid sessp.Tclient, sid sessp.Tsession) *Session {
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

func (st *SessionTable) ProcessHeartbeats(hbs *sp.Theartbeat) {
	st.Lock()
	defer st.Unlock()

	for sid, _ := range hbs.Sids {
		sess := st.allocL(0, sessp.Tsession(sid))
		sess.Lock()
		if !sess.closed {
			sess.heartbeatL(hbs)
		}
		sess.Unlock()
	}
}

func (st *SessionTable) SessThread(sid sessp.Tsession) *threadmgr.ThreadMgr {
	if sess, ok := st.Lookup(sid); ok {
		return sess.threadmgr
	} else {
		db.DFatalf("SessThread: no thread for %v\n", sid)
	}
	return nil
}

func (st *SessionTable) KillSessThread(sid sessp.Tsession) {
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
