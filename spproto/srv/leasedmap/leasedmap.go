// Package leasedmap maintains a map of leased files
package leasedmap

import (
	"sync"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type Entry struct {
	P      sp.Tpath
	Name   string
	Obj    fs.FsObj
	Parent fs.Dir
}

type LeasedMap struct {
	sync.Mutex
	ps   map[sp.Tpath]sp.TleaseId
	lids map[sp.TleaseId][]*Entry
}

func NewLeasedMap() *LeasedMap {
	et := &LeasedMap{
		ps:   make(map[sp.Tpath]sp.TleaseId),
		lids: make(map[sp.TleaseId][]*Entry),
	}
	return et
}

func (lm *LeasedMap) Insert(p sp.Tpath, lid sp.TleaseId, n string, o fs.FsObj, dir fs.Dir) {
	lm.Lock()
	defer lm.Unlock()

	_, ok := lm.ps[p]
	if ok {
		db.DFatalf("Insert %v exists %q\n", p, lm.ps)
	}
	lm.ps[p] = lid
	v, ok := lm.lids[lid]
	if !ok {
		lm.lids[lid] = []*Entry{&Entry{p, n, o, dir}}
	} else {
		lm.lids[lid] = append(v, &Entry{p, n, o, dir})
	}
	db.DPrintf(db.LEASESRV, "Insert %q %v %v\n", p, lid, lm.lids)
}

func (lm *LeasedMap) Delete(p sp.Tpath) bool {
	lm.Lock()
	defer lm.Unlock()

	lid, ok := lm.ps[p]
	if !ok {
		db.DPrintf(db.LEASESRV, "Delete %v doesn't exist %v\n", p, lm.ps)
		return false
	}
	delete(lm.ps, p)
	for i, e := range lm.lids[lid] {
		if e.P == p {
			lm.lids[lid] = append(lm.lids[lid][:i], lm.lids[lid][i+1:]...)
			break
		}
	}
	db.DPrintf(db.LEASESRV, "Delete %q %v\n", p, lm.lids)
	return true
}

func (lm *LeasedMap) Rename(p sp.Tpath, dst string) bool {
	lm.Lock()
	defer lm.Unlock()

	lid, ok := lm.ps[p]
	if !ok {
		db.DFatalf("Rename %v doesn't exist %v\n", p, lm.ps)
		return false
	}
	for _, v := range lm.lids[lid] {
		if v.P == p {
			v.Name = dst
			break
		}
	}
	db.DPrintf(db.LEASESRV, "Rename %v %q %v\n", p, dst, lm.lids)
	return true
}

func (lm *LeasedMap) Expired(lid sp.TleaseId) []*Entry {
	lm.Lock()
	defer lm.Unlock()
	ps, _ := lm.lids[lid]
	delete(lm.lids, lid)
	return ps
}
