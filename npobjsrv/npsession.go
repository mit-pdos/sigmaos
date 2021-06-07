package npobjsrv

import (
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type SessionTable struct {
	mu            sync.Mutex
	sessions      map[np.Tsession]bool
	fidTables     map[np.Tsession]map[np.Tfid]*Fid
	fidTableLocks map[np.Tsession]*sync.Mutex
}

func MakeSessionTable() *SessionTable {
	st := &SessionTable{}
	st.sessions = make(map[np.Tsession]bool)
	st.fidTables = make(map[np.Tsession]map[np.Tfid]*Fid)
	st.fidTableLocks = map[np.Tsession]*sync.Mutex{}
	return st
}

func (st *SessionTable) RegisterSession(sess np.Tsession) {
	db.DLPrintf("SETAB", "Register session %v", sess)

	st.mu.Lock()
	defer st.mu.Unlock()

	st.sessions[sess] = true
	if _, ok := st.fidTables[sess]; !ok {
		st.fidTables[sess] = map[np.Tfid]*Fid{}
	}
	if _, ok := st.fidTableLocks[sess]; !ok {
		st.fidTableLocks[sess] = &sync.Mutex{}
	}
}

func (st *SessionTable) lookupFid(sess np.Tsession, fid np.Tfid) (*Fid, bool) {
	db.DLPrintf("SETAB", "lookupFid %v %v", sess, fid)

	st.mu.Lock()
	tab := st.fidTables[sess]
	st.mu.Unlock()

	tabLock := st.fidTableLock(sess)
	tabLock.Lock()
	defer tabLock.Unlock()

	f, ok := tab[fid]
	return f, ok
}

func (st *SessionTable) addFid(sess np.Tsession, fid np.Tfid, f *Fid) {
	db.DLPrintf("SETAB", "addFid %v %v %v", sess, fid, f)

	st.mu.Lock()
	tab := st.fidTables[sess]
	st.mu.Unlock()

	tabLock := st.fidTableLock(sess)
	tabLock.Lock()
	defer tabLock.Unlock()

	tab[fid] = f
}

func (st *SessionTable) delFid(sess np.Tsession, fid np.Tfid) NpObj {
	db.DLPrintf("SETAB", "delFid %v %v", sess, fid)

	st.mu.Lock()
	tab := st.fidTables[sess]
	st.mu.Unlock()

	tabLock := st.fidTableLock(sess)
	tabLock.Lock()
	defer tabLock.Unlock()

	o := tab[fid].obj
	delete(tab, fid)
	return o
}

func (st *SessionTable) IterateFids(fi func(*Fid)) {
	st.mu.Lock()
	defer st.mu.Unlock()

	for sess, tab := range st.fidTables {
		tabLock := st.fidTableLockL(sess)
		tabLock.Lock()
		for _, f := range tab {
			fi(f)
		}
		tabLock.Unlock()
	}
}

func (st *SessionTable) fidTableLock(sess np.Tsession) *sync.Mutex {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.fidTableLockL(sess)
}

func (st *SessionTable) fidTableLockL(sess np.Tsession) *sync.Mutex {
	if l, ok := st.fidTableLocks[sess]; ok {
		return l
	} else {
		log.Fatalf("Fid table lock not found")
	}
	return nil
}
