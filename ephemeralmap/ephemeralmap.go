package ephemeralmap

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// XXX maintain lid -> string map
type EphemeralMap struct {
	sync.Mutex
	pns  map[string]sp.TleaseId
	lids map[sp.TleaseId][]string
}

func NewEphemeralMap() *EphemeralMap {
	et := &EphemeralMap{
		pns:  make(map[string]sp.TleaseId),
		lids: make(map[sp.TleaseId][]string),
	}
	return et
}

func (et *EphemeralMap) Insert(pn string, lid sp.TleaseId) {
	et.Lock()
	defer et.Unlock()

	lid, ok := et.pns[pn]
	if ok {
		db.DFatalf("Insert %v exists %v\n", pn, et.pns)
	}
	et.pns[pn] = lid
	v, ok := et.lids[lid]
	if !ok {
		et.lids[lid] = []string{pn}
	} else {
		et.lids[lid] = append(v, pn)
	}
}

func (et *EphemeralMap) Delete(pn string) {
	et.Lock()
	defer et.Unlock()

	lid, ok := et.pns[pn]
	if !ok {
		db.DPrintf(db.ALWAYS, "%v: Delete %v doesn't exist %v\n", proc.GetName(), pn, et.pns)
	}
	delete(et.pns, pn)
	for i, v := range et.lids[lid] {
		if v == pn {
			et.lids[lid] = append(et.lids[lid][:i], et.lids[lid][i+1:]...)
			break
		}
	}
}

func (et *EphemeralMap) Rename(src, dst string) {
	et.Lock()
	defer et.Unlock()

	lid, ok := et.pns[src]
	if !ok {
		db.DFatalf("Rename src %v doesn't exist %v\n", src, et.pns)
	}
	delete(et.pns, src)
	et.pns[dst] = lid
	for i, v := range et.lids[lid] {
		if v == src {
			et.lids[lid][i] = dst
			break
		}
	}
}

func (et *EphemeralMap) Expire(lid sp.TleaseId) []string {
	et.Lock()
	defer et.Unlock()
	pns, ok := et.lids[lid]
	if ok {
		for _, pn := range pns {
			delete(et.pns, pn)
		}
	}
	delete(et.lids, lid)
	return pns
}
