package sesssrv

import (
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	// db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const NLAST = 10

type sessionTable struct {
	newSess NewSessionI
	mu      sync.RWMutex
	//	deadlock.Mutex
	sessions  map[sessp.Tsession]*Session
	lasts     map[sessp.Tsession]*Session // for testing
	lastClnts map[sp.TclntId]*Session     // for testing
}

func newSessionTable(newSess NewSessionI) *sessionTable {
	st := &sessionTable{newSess: newSess}
	st.sessions = make(map[sessp.Tsession]*Session)
	st.lasts = make(map[sessp.Tsession]*Session)
	st.lastClnts = make(map[sp.TclntId]*Session)
	return st
}

func (st *sessionTable) CloseSessions() error {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for _, sess := range st.sessions {
		sess.Close()
	}
	return nil
}

func (st *sessionTable) Lookup(sid sessp.Tsession) (*Session, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	sess, ok := st.sessions[sid]
	return sess, ok
}

func (st *sessionTable) Alloc(p *sp.Tprincipal, sid sessp.Tsession, nc *netConn) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()

	if sess, ok := st.sessions[sid]; ok {
		return sess
	}
	sess := newSession(st.newSess.NewSession(p, sid), sid, nc)
	st.sessions[sid] = sess
	if len(st.lasts) < NLAST {
		st.lasts[sid] = sess
	}
	return sess
}

// Return a last session
func (st *sessionTable) lastSession() *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var sess *Session
	for _, s := range st.lasts {
		sess = s
		break
	}
	return sess
}

func (st *sessionTable) AddLastClnt(cid sp.TclntId, sess *Session) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.lastClnts) < NLAST {
		st.lastClnts[cid] = sess
	}
}

func (st *sessionTable) DelLastClnt(cid sp.TclntId) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.lastClnts[cid]; ok {
		delete(st.lastClnts, cid)
	}
}

// Return a last clnt
func (st *sessionTable) lastClnt() (sp.TclntId, *Session) {
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
