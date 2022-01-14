package fsobjsrv

import (
	"sync"

	"ulambda/fid"
	"ulambda/fs"
)

type ephemeralTable struct {
	sync.Mutex
	detached  bool // Marks whether or not the session has already detached.
	ephemeral map[fs.FsObj]*fid.Fid
}

func makeEphemeralTable() *ephemeralTable {
	ft := &ephemeralTable{}
	ft.detached = false
	ft.ephemeral = make(map[fs.FsObj]*fid.Fid)
	return ft
}

// If the session already detached, do nothing & return false. Otherwise, add
// to the ephemeral table.
func (et *ephemeralTable) Add(o fs.FsObj, f *fid.Fid) bool {
	et.Lock()
	defer et.Unlock()
	if et.detached {
		return false
	}
	et.ephemeral[o] = f
	return true
}

func (et *ephemeralTable) Del(o fs.FsObj) {
	et.Lock()
	defer et.Unlock()
	delete(et.ephemeral, o)
}

func (et *ephemeralTable) Get() map[fs.FsObj]*fid.Fid {
	et.Lock()
	defer et.Unlock()

	e := make(map[fs.FsObj]*fid.Fid)

	// XXX Making a full copy may be overkill...
	for o, f := range et.ephemeral {
		e[o] = f
	}

	return e
}

func (et *ephemeralTable) Detach() {
	et.Lock()
	defer et.Unlock()
	et.detached = true
}
