package fsetcd

import (
	"github.com/hashicorp/golang-lru/v2"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

const N = 8192

type dcEntry struct {
	dir  *DirInfo
	v    sp.TQversion
	stat bool
}

type Dcache struct {
	c *lru.Cache[sp.Tpath, *dcEntry]
}

func newDcache() *Dcache {
	c, err := lru.New[sp.Tpath, *dcEntry](N)
	if err != nil {
		db.DFatalf("newDcache err %v\n", err)
	}
	return &Dcache{c: c}
}

func (dc *Dcache) lookup(d sp.Tpath) (*DirInfo, sp.TQversion, bool, bool) {
	de, ok := dc.c.Get(d)
	if ok {
		db.DPrintf(db.FSETCD, "Lookup dcache hit %v %v", d, de)
		return de.dir, de.v, de.stat, ok
	}
	return nil, 0, false, false
}

func (dc *Dcache) insert(d sp.Tpath, dir *DirInfo, v sp.TQversion, stat bool) {
	db.DPrintf(db.FSETCD, "Insert dcache %v %v", d, dir)
	if evict := dc.c.Add(d, &dcEntry{dir, v, stat}); evict {
		db.DPrintf(db.FSETCD, "Eviction")
	}
}

// d might not be in the cache since it maybe uncacheable
func (dc *Dcache) update(d sp.Tpath, dir *DirInfo) bool {
	de, ok := dc.c.Get(d)
	if ok {
		db.DPrintf(db.FSETCD, "Update dcache %v %v %v", d, dir, de.v+1)
		de.dir = dir
		de.v += 1
		return true
	}
	return false
}
