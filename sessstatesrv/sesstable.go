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
	mu sync.RWMutex
	//	deadlock.Mutex
	newps     sps.MkProtServer
	sesssrv  sps.SessServer
	sessions map[sessp.Tsession]*Session
	last     *Session // for tests
	attachf  sps.AttachClntF
	detachf  sps.DetachClntF
}

func NewSessionTable(newps sps.MkProtServer, sesssrv sps.SessServer, attachf sps.AttachClntF, detachf sps.DetachClntF) *SessionTable {
	st := &SessionTable{sesssrv: sesssrv, newps: newps, attachf: attachf, detachf: detachf}
	st.sessions = make(map[sessp.Tsession]*Session)
	return st
}

func (st *SessionTable) CloseSessions() error {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for _, sess := range st.sessions {
		sess.Close()
	}
	return nil
}

func (st *SessionTable) Lookup(sid sessp.Tsession) (*Session, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *SessionTable) Alloc(cid sessp.Tclient, sid sessp.Tsession) *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()

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
				st.mu.RUnlock()
				st.mu.Lock()
				// Make sure to re-lock the reader lock, as the caller expects it to be
				// held.
				defer st.mu.RLock()
				// Defer statements happen in LIFO order, so make sure to unlock the
				// writer lock before the reader lock is taken.
				defer st.mu.Unlock()
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
	sess := newSession(st.newps(st.sesssrv, sid), cid, sid, st.attachf, st.detachf)
	// sess := newSession(st.newps(st.sesssrv, sid), cid, sid, st.tm.AddThread(), st.attachf, st.detachf)
	st.sessions[sid] = sess
	st.last = sess
	return sess
}

func (st *SessionTable) ProcessHeartbeats(hbs *sp.Theartbeat) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for sid, _ := range hbs.Sids {
		sess := st.allocRL(0, sessp.Tsession(sid))
		sess.Lock()
		if !sess.closed {
			sess.heartbeatL(hbs)
		}
		sess.Unlock()
	}
}

// For when using a thread manager to execute requests of a session sequentially
func (st *SessionTable) SessThread(sid sessp.Tsession) *threadmgr.ThreadMgr {
	if _, ok := st.Lookup(sid); ok {
		return nil
	} else {
		db.DFatalf("SessThread: no thread for %v\n", sid)
	}
	return nil
}

// Note: Used when running sessions using threadmgr
func (st *SessionTable) KillSessThread(sid sessp.Tsession) {
	//t := st.SessThread(sid)
	//st.mu.RLock()
	//defer st.mu.RUnlock()
	// st.tm.RemoveThread(t)
}

func (st *SessionTable) LastSession() *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.last != nil {
		return st.last
	}
	return nil
}
