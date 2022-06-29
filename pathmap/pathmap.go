package pathmap

import (
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Entry struct {
	n int
	E interface{}
}

func mkEntry(i interface{}) *Entry {
	e := &Entry{}
	e.n = 1
	e.E = i
	return e
}

type PathMap struct {
	sync.Mutex
	paths map[string]*Entry
}

func MkPathMap() *PathMap {
	pm := &PathMap{}
	pm.paths = make(map[string]*Entry)
	return pm
}

func (pm *PathMap) Lookup(p np.Path) (*Entry, bool) {
	pm.Lock()
	defer pm.Unlock()

	if ne, ok := pm.paths[p.String()]; ok {
		return ne, true
	}
	return nil, false
}

func (pm *PathMap) Insert(p np.Path, e interface{}) *Entry {
	pm.Lock()
	defer pm.Unlock()

	ne, ok := pm.paths[p.String()]
	if ok {
		ne.n += 1
		// log.Printf("insert %v %v\n", p, ne)
		return ne
	}
	ne = mkEntry(e)
	log.Printf("new insert %v\n", ne)
	pm.paths[p.String()] = ne
	return ne
}

func (pm *PathMap) Delete(p np.Path) {
	pm.Lock()
	defer pm.Unlock()

	ne, ok := pm.paths[p.String()]
	if !ok {
		db.DFatalf("delete %v\n", p)
	}
	ne.n -= 1
	if ne.n <= 0 {
		log.Printf("delete %v\n", p)
		delete(pm.paths, p.String())
	}
}
