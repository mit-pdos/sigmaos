package fsetcd

import (
	"github.com/hashicorp/golang-lru/v2"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type dcEntry struct {
	dir  *DirInfo
	v    sp.TQversion
	stat bool
}

type Dcache struct {
	c *lru.Cache[sp.Tpath, *dcEntry]
}

func newDcache() *Dcache {
	c, err := lru.New[sp.Tpath, *dcEntry](8192)
	if err != nil {
		db.DFatalf("newDcache err %v\n", err)
	}
	return &Dcache{c: c}
}

func (dc *Dcache) Lookup(d sp.Tpath) (*DirInfo, sp.TQversion, bool, bool) {
	de, ok := dc.c.Get(d)
	if ok {
		db.DPrintf(db.FSETCD, "Lookup dcache hit %v %v", d, de)
		return de.dir, de.v, de.stat, ok
	}
	return nil, 0, false, false
}

func (dc *Dcache) Insert(d sp.Tpath, dir *DirInfo, v sp.TQversion, stat bool) {
	db.DPrintf(db.FSETCD, "Insert dcache %v %v", d, dir)
	if evict := dc.c.Add(d, &dcEntry{dir, v, stat}); evict {
		db.DPrintf(db.FSETCD, "Eviction")
	}
}

func (dc *Dcache) Update(d sp.Tpath, dir *DirInfo) {
	de, ok := dc.c.Get(d)
	if ok {
		db.DPrintf(db.FSETCD, "Update dcache %v %v %v", d, dir, de.v+1)
		de.dir = dir
		de.v += 1
		return
	}
	db.DFatalf("Update dcache: key %v isn't present", d)
}
