package sessstatesrv

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	// db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type SessionTable struct {
	mu sync.RWMutex
	//	deadlock.Mutex
	newps    sps.NewProtServer
	sesssrv  sps.SessServer
	sessions map[sessp.Tsession]*Session
	last     *Session // for tests
	attachf  sps.AttachClntF
	detachf  sps.DetachClntF
}

func NewSessionTable(newps sps.NewProtServer, sesssrv sps.SessServer, attachf sps.AttachClntF, detachf sps.DetachClntF) *SessionTable {
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

func (st *SessionTable) Alloc(sid sessp.Tsession) *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()

	return st.allocRL(sid)
}

func (st *SessionTable) allocRL(sid sessp.Tsession) *Session {
	// Loop to first try with reader lock, then retry with writer lock.
	for i := 0; i < 2; i++ {
		if sess, ok := st.sessions[sid]; ok {
			sess.Lock()
			defer sess.Unlock()
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
	sess := newSession(st.newps(st.sesssrv, sid), sid, st.attachf, st.detachf)
	st.sessions[sid] = sess
	st.last = sess
	return sess
}

func (st *SessionTable) ProcessHeartbeats(hbs *sp.Theartbeat) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for sid, _ := range hbs.Sids {
		sess := st.allocRL(sessp.Tsession(sid))
		sess.Lock()
		if !sess.closed {
			sess.heartbeatL(hbs)
		}
		sess.Unlock()
	}
}

func (st *SessionTable) AddClnt(sid sessp.Tsession, cid sp.TclntId) bool {
	sess, ok := st.Lookup(sid)
	if !ok {
		return false
	}
	sess.AddClnt(cid)
	return true
}

func (st *SessionTable) CloseClnt(sid sessp.Tsession, cid sp.TclntId) bool {
	sess, ok := st.Lookup(sid)
	if !ok {
		return false
	}
	sess.CloseClnt(cid)
	return true
}

func (st *SessionTable) LastSession() *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.last != nil {
		return st.last
	}
	return nil
}
