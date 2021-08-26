package session

import (
	"sync"

	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	np "ulambda/ninep"
)

type Session struct {
	mu        sync.Mutex
	fids      map[np.Tfid]*fid.Fid
	ephemeral map[fs.FsObj]*fid.Fid
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
		new.fids = make(map[np.Tfid]*fid.Fid)
		new.ephemeral = make(map[fs.FsObj]*fid.Fid)
		st.sessions[id] = new
	}
}

func (st *SessionTable) LookupFid(id np.Tsession, fid np.Tfid) (*fid.Fid, bool) {
	db.DLPrintf("SETAB", "lookupFid %v %v", id, fid)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	f, ok := sess.fids[fid]
	return f, ok
}

func (st *SessionTable) AddFid(id np.Tsession, fid np.Tfid, f *fid.Fid) {
	db.DLPrintf("SETAB", "addFid %v %v %v", id, fid, f)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.fids[fid] = f
}

func (st *SessionTable) DelFid(id np.Tsession, fid np.Tfid) fs.FsObj {
	db.DLPrintf("SETAB", "delFid %v %v", id, fid)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	o := sess.fids[fid].ObjU()
	delete(sess.fids, fid)
	return o
}

func (st *SessionTable) AddEphemeral(id np.Tsession, o fs.FsObj, f *fid.Fid) {
	db.DLPrintf("SETAB", "addEphemeral %v %v %v", id, o, f)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.ephemeral[o] = f
}

func (st *SessionTable) DelEphemeral(id np.Tsession, o fs.FsObj) {
	db.DLPrintf("SETAB", "delEpehemeral %v %v", id, o)

	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	delete(sess.ephemeral, o)
}

func (st *SessionTable) GetEphemeral(id np.Tsession) map[fs.FsObj]*fid.Fid {
	st.mu.Lock()
	sess := st.sessions[id]
	st.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()

	// XXX Making a full copy may be overkill...
	e := make(map[fs.FsObj]*fid.Fid)
	for o, f := range sess.ephemeral {
		e[o] = f
	}

	return e
}

func (st *SessionTable) IterateFids(fi func(*fid.Fid)) {
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
