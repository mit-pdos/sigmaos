package protsrv

import (
	"sync"

	"sigmaos/fid"
	"sigmaos/path"
)

type ephemeralTable struct {
	sync.Mutex
	ephemeral map[string]*fid.Pobj
}

func makeEphemeralTable() *ephemeralTable {
	ft := &ephemeralTable{}
	ft.ephemeral = make(map[string]*fid.Pobj)
	return ft
}

func (et *ephemeralTable) Add(p path.Path, po *fid.Pobj) {
	et.Lock()
	defer et.Unlock()
	et.ephemeral[p.String()] = po
}

func (et *ephemeralTable) Del(p path.Path) {
	et.Lock()
	defer et.Unlock()
	delete(et.ephemeral, p.String())
}

func (et *ephemeralTable) Rename(s, t path.Path, po *fid.Pobj) {
	et.Lock()
	defer et.Unlock()
	_, ok := et.ephemeral[s.String()]
	if ok {
		delete(et.ephemeral, s.String())
		et.ephemeral[t.String()] = po
	}
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
