package protsrv

import (
	"sync"

	"ulambda/fid"
	"ulambda/fs"
)

type ephemeralTable struct {
	sync.Mutex
	ephemeral map[fs.FsObj]*fid.Fid
}

func makeEphemeralTable() *ephemeralTable {
	ft := &ephemeralTable{}
	ft.ephemeral = make(map[fs.FsObj]*fid.Fid)
	return ft
}

func (et *ephemeralTable) Add(o fs.FsObj, f *fid.Fid) {
	et.Lock()
	defer et.Unlock()
	et.ephemeral[o] = f
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
