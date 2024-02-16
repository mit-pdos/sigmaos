package sesssrv

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	// db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type SessionTable struct {
	newSess NewSessionI
	mu      sync.RWMutex
	//	deadlock.Mutex
	sessions  map[sessp.Tsession]*Session
	lasts     map[sessp.Tsession]*Session   // for testing
	lastClnts map[sp.TclntId]sessp.Tsession // for testing
}

func NewSessionTable(newSess NewSessionI) *SessionTable {
	st := &SessionTable{newSess: newSess}
	st.sessions = make(map[sessp.Tsession]*Session)
	st.lasts = make(map[sessp.Tsession]*Session)
	st.lastClnts = make(map[sp.TclntId]sessp.Tsession)
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
	sess := newSession(st.newSess.NewSession(sid), sid)
	st.sessions[sid] = sess
	if len(st.lasts) < NLAST {
		st.lasts[sid] = sess
	}
	return sess
}

// Return a last session
func (st *SessionTable) lastSession() *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var sess *Session
	for _, s := range st.lasts {
		sess = s
		break
	}
	return sess
}

func (st *SessionTable) AddLastClnt(cid sp.TclntId, sid sessp.Tsession) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.lastClnts) < NLAST {
		st.lastClnts[cid] = sid
	}
}

func (st *SessionTable) DelLastClnt(cid sp.TclntId) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.lastClnts[cid]; ok {
		delete(st.lastClnts, cid)
	}
}

// Return a last clnt
func (st *SessionTable) lastClnt() (sp.TclntId, sessp.Tsession) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	c := sp.NoClntId
	s := sessp.Tsession(0)
	for cid, sid := range st.lastClnts {
		c = cid
		s = sid
		break
	}
	if c != sp.NoClntId {
		delete(st.lastClnts, c)
	}
	return c, s
}
