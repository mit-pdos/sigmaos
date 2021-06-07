package npobjsrv

import (
	"sync"

	np "ulambda/ninep"
)

type SessionTable struct {
	mu        sync.Mutex
	sessions  map[np.Tsession]bool
	fidTables map[np.Tsession]map[np.Tfid]*Fid
}

func MakeSessionTable() *SessionTable {
	st := &SessionTable{}
	st.sessions = map[np.Tsession]bool{}
	st.fidTables = map[np.Tsession]map[np.Tfid]*Fid{}
	return st
}
