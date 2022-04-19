package protsrv

import (
	"sync"

	"ulambda/fid"
	"ulambda/fs"
)

type ephemeralTable struct {
	sync.Mutex
	ephemeral map[fs.FsObj]*fid.Pobj
}

func makeEphemeralTable() *ephemeralTable {
	ft := &ephemeralTable{}
	ft.ephemeral = make(map[fs.FsObj]*fid.Pobj)
	return ft
}

func (et *ephemeralTable) Add(o fs.FsObj, po *fid.Pobj) {
	et.Lock()
	defer et.Unlock()
	et.ephemeral[o] = po
}

func (et *ephemeralTable) Del(o fs.FsObj) {
	et.Lock()
	defer et.Unlock()
	delete(et.ephemeral, o)
}

func (et *ephemeralTable) Get() []*fid.Pobj {
	et.Lock()
	defer et.Unlock()

	e := make([]*fid.Pobj, 0, len(et.ephemeral))
	for _, po := range et.ephemeral {
		e = append(e, po)
	}

	return e
}
