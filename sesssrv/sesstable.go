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
	lasts     map[sessp.Tsession]*Session // for testing
	lastClnts map[sp.TclntId]*Session     // for testing
}

func NewSessionTable(newSess NewSessionI) *SessionTable {
	st := &SessionTable{newSess: newSess}
	st.sessions = make(map[sessp.Tsession]*Session)
	st.lasts = make(map[sessp.Tsession]*Session)
	st.lastClnts = make(map[sp.TclntId]*Session)
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

func (st *SessionTable) Alloc(sid sessp.Tsession, nc *netConn) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()

	if sess, ok := st.sessions[sid]; ok {
		return sess
	}
	sess := newSession(st.newSess.NewSession(sid), sid, nc)
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

func (st *SessionTable) AddLastClnt(cid sp.TclntId, sess *Session) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.lastClnts) < NLAST {
		st.lastClnts[cid] = sess
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
func (st *SessionTable) lastClnt() (sp.TclntId, *Session) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	c := sp.NoClntId
	var sess *Session
	for cid, s := range st.lastClnts {
		c = cid
		sess = s
		break
	}
	if c != sp.NoClntId {
		delete(st.lastClnts, c)
	}
	return c, sess
}
