package ephemeralmap

import (
	"sync"

	db "sigmaos/debug"
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

	_, ok := et.pns[pn]
	if ok {
		db.DFatalf("Insert %v exists %q\n", pn, et.pns)
	}
	et.pns[pn] = lid
	v, ok := et.lids[lid]
	if !ok {
		et.lids[lid] = []string{pn}
	} else {
		et.lids[lid] = append(v, pn)
	}
	db.DPrintf(db.LEASESRV, "Insert %q %v %v\n", pn, lid, et.lids)
}

func (et *EphemeralMap) Delete(pn string) bool {
	et.Lock()
	defer et.Unlock()

	lid, ok := et.pns[pn]
	if !ok {
		db.DPrintf(db.LEASESRV, "Delete %v doesn't exist %v\n", pn, et.pns)
		return false
	}
	delete(et.pns, pn)
	for i, v := range et.lids[lid] {
		if v == pn {
			et.lids[lid] = append(et.lids[lid][:i], et.lids[lid][i+1:]...)
			break
		}
	}
	db.DPrintf(db.LEASESRV, "Delete %q %v\n", pn, et.lids)
	return true
}

func (et *EphemeralMap) Rename(src, dst string) bool {
	et.Lock()
	defer et.Unlock()

	lid, ok := et.pns[src]
	if !ok {
		db.DFatalf("Rename src %v doesn't exist %v\n", src, et.pns)
		return false
	}
	delete(et.pns, src)
	et.pns[dst] = lid
	for i, v := range et.lids[lid] {
		if v == src {
			et.lids[lid][i] = dst
			break
		}
	}
	db.DPrintf(db.LEASESRV, "Rename %q %q %v\n", src, dst, et.lids)
	return true
}

func (et *EphemeralMap) Expired(lid sp.TleaseId) []string {
	et.Lock()
	defer et.Unlock()
	pns, _ := et.lids[lid]
	delete(et.lids, lid)
	return pns
}
