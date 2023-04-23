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
	sync.RWMutex
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
	return st
}

func (st *SessionTable) QueueLen() int64 {
	st.RLock()
	defer st.RUnlock()
	len := int64(0)
	for _, s := range st.sessions {
		len += s.QueueLen()
	}
	return len
}

func (st *SessionTable) Lookup(sid sessp.Tsession) (*Session, bool) {
	st.RLock()
	defer st.RUnlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) Alloc(cid sessp.Tclient, sid sessp.Tsession) *Session {
	st.RLock()
	defer st.RUnlock()

	return st.allocRL(cid, sid)
}

func (st *SessionTable) allocRL(cid sessp.Tclient, sid sessp.Tsession) *Session {
	// Loop to first try with reader lock, then retry with writer lock.
	for i := 0; i < 2; i++ {
		if sess, ok := st.sessions[sid]; ok {
			sess.Lock()
			defer sess.Unlock()
			if sess.ClientId == 0 {
				sess.ClientId = cid
			}
			return sess
		} else {
			if i == 0 {
				// Session not in the session table. Upgrade to write lock.
				st.RUnlock()
				st.Lock()
				// Make sure to re-lock the reader lock, as the caller expects it to be
				// held.
				defer st.RLock()
				// Defer statements happen in LIFO order, so make sure to unlock the
				// writer lock before the reader lock is taken.
				defer st.Unlock()
				// Retry to see if the session is now in the table. This may happen if
				// between releasing the reader lock and grabbing the writer lock another
				// caller sneaked in, grabbed the writer lock, and allocated the session.
				continue
			} else {
				// Second pass was unsuccessful. Continue to allocation.
				break
			}
		}
	}
	sess := makeSession(st.mkps(st.sesssrv, sid), cid, sid, st.tm.AddThread())
	st.sessions[sid] = sess
	st.last = sess
	return sess
}

func (st *SessionTable) ProcessHeartbeats(hbs *sp.Theartbeat) {
	st.RLock()
	defer st.RUnlock()

	for sid, _ := range hbs.Sids {
		sess := st.allocRL(0, sessp.Tsession(sid))
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
	st.RLock()
	defer st.RUnlock()
	st.tm.RemoveThread(t)
}

func (st *SessionTable) LastSession() *Session {
	st.RLock()
	defer st.RUnlock()
	if st.last != nil {
		return st.last
	}
	return nil
}
