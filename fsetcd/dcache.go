package fsetcd

import (
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type dcEntry struct {
	dir  *DirInfo
	v    sp.TQversion
	stat bool
}

type Dcache struct {
	sync.Mutex
	dcache map[sp.Tpath]*dcEntry
}

func newDcache() *Dcache {
	return &Dcache{dcache: make(map[sp.Tpath]*dcEntry)}
}

func (dc *Dcache) Lookup(d sp.Tpath) (*DirInfo, sp.TQversion, bool, bool) {
	dc.Lock()
	defer dc.Unlock()

	de, ok := dc.dcache[d]
	if ok {
		db.DPrintf(db.FSETCD, "Lookup hit %v %v", d, de)
		return de.dir, de.v, de.stat, ok
	}
	return nil, 0, false, false
}

func (dc *Dcache) Insert(d sp.Tpath, dir *DirInfo, v sp.TQversion, stat bool) {
	dc.Lock()
	defer dc.Unlock()

	db.DPrintf(db.FSETCD, "Insert %v %v", d, dir)
	dc.dcache[d] = &dcEntry{dir, v, stat}
}

func (dc *Dcache) Invalidate(d sp.Tpath) {
	dc.Lock()
	defer dc.Unlock()

	db.DPrintf(db.FSETCD, "Invalidate %v", d)
	delete(dc.dcache, d)
}
