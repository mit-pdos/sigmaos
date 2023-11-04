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
		db.DPrintf(db.FSETCD, "Lookup dcache hit %v %v", d, de)
		return de.dir, de.v, de.stat, ok
	}
	return nil, 0, false, false
}

func (dc *Dcache) Insert(d sp.Tpath, dir *DirInfo, v sp.TQversion, stat bool) {
	dc.Lock()
	defer dc.Unlock()

	db.DPrintf(db.FSETCD, "Insert dcache %v %v", d, dir)
	dc.dcache[d] = &dcEntry{dir, v, stat}
}

func (dc *Dcache) Update(d sp.Tpath, dir *DirInfo) {
	dc.Lock()
	defer dc.Unlock()

	if de, ok := dc.dcache[d]; ok {
		db.DPrintf(db.FSETCD, "Update dcache %v %v %v", d, dir, de.v+1)
		de.dir = dir
		de.v += 1
		return
	}
	db.DFatalf("Update dcache: key %v isn't present", d)
}

func (dc *Dcache) Invalidate(d sp.Tpath) {
	dc.Lock()
	defer dc.Unlock()

	db.DPrintf(db.FSETCD, "Invalidate dcache %v", d)
	delete(dc.dcache, d)
}
