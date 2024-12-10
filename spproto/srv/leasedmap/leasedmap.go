// Package leasedmap maintains a map of leased files
package leasedmap

import (
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type LeasedMap struct {
	sync.Mutex
	pns  map[string]sp.TleaseId
	lids map[sp.TleaseId][]string
}

func NewLeasedMap() *LeasedMap {
	et := &LeasedMap{
		pns:  make(map[string]sp.TleaseId),
		lids: make(map[sp.TleaseId][]string),
	}
	return et
}

func (lm *LeasedMap) Insert(pn string, lid sp.TleaseId) {
	lm.Lock()
	defer lm.Unlock()

	_, ok := lm.pns[pn]
	if ok {
		db.DFatalf("Insert %v exists %q\n", pn, lm.pns)
	}
	lm.pns[pn] = lid
	v, ok := lm.lids[lid]
	if !ok {
		lm.lids[lid] = []string{pn}
	} else {
		lm.lids[lid] = append(v, pn)
	}
	db.DPrintf(db.LEASESRV, "Insert %q %v %v\n", pn, lid, lm.lids)
}

func (lm *LeasedMap) Delete(pn string) bool {
	lm.Lock()
	defer lm.Unlock()

	lid, ok := lm.pns[pn]
	if !ok {
		db.DPrintf(db.LEASESRV, "Delete %v doesn't exist %v\n", pn, lm.pns)
		return false
	}
	delete(lm.pns, pn)
	for i, v := range lm.lids[lid] {
		if v == pn {
			lm.lids[lid] = append(lm.lids[lid][:i], lm.lids[lid][i+1:]...)
			break
		}
	}
	db.DPrintf(db.LEASESRV, "Delete %q %v\n", pn, lm.lids)
	return true
}

func (lm *LeasedMap) Rename(src, dst string) bool {
	lm.Lock()
	defer lm.Unlock()

	lid, ok := lm.pns[src]
	if !ok {
		db.DFatalf("Rename src %v doesn't exist %v\n", src, lm.pns)
		return false
	}
	delete(lm.pns, src)
	lm.pns[dst] = lid
	for i, v := range lm.lids[lid] {
		if v == src {
			lm.lids[lid][i] = dst
			break
		}
	}
	db.DPrintf(db.LEASESRV, "Rename %q %q %v\n", src, dst, lm.lids)
	return true
}

func (lm *LeasedMap) Expired(lid sp.TleaseId) []string {
	lm.Lock()
	defer lm.Unlock()
	pns, _ := lm.lids[lid]
	delete(lm.lids, lid)
	return pns
}
