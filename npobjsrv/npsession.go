package npobjsrv

import (
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Session struct {
	mu   sync.Mutex
	fids map[np.Tfid]*Fid
}

type SessionTable struct {
	mu       sync.Mutex
	sessions map[np.Tsession]*Session
}

func MakeSessionTable() *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]*Session)
	return st
}

func (st *SessionTable) RegisterSession(id np.Tsession) {
	db.DLPrintf("SETAB", "Register session %v", id)

	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.sessions[id]; !ok {
		new := &Session{}
		new.fids = make(map[np.Tfid]*Fid)
		st.sessions[id] = new
	}
}

func (st *SessionTable) lookupFid(id np.Tsession, fid np.Tfid) (*Fid, bool) {
	db.DLPrintf("SETAB", "lookupFid %v %v", id, fid)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	f, ok := sess.fids[fid]
	return f, ok
}

func (st *SessionTable) addFid(id np.Tsession, fid np.Tfid, f *Fid) {
	db.DLPrintf("SETAB", "addFid %v %v %v", id, fid, f)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.fids[fid] = f
}

func (st *SessionTable) delFid(id np.Tsession, fid np.Tfid) NpObj {
	db.DLPrintf("SETAB", "delFid %v %v", id, fid)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	o := sess.fids[fid].obj
	delete(sess.fids, fid)
	return o
}

func (st *SessionTable) IterateFids(fi func(*Fid)) {
	st.mu.Lock()
	defer st.mu.Unlock()

	for _, session := range st.sessions {
		session.mu.Lock()
		for _, f := range session.fids {
			fi(f)
		}
		session.mu.Unlock()
	}
}
