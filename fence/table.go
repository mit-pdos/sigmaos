package fence

import (
	"fmt"
	"log"
	"sync"

	np "ulambda/ninep"
)

//
// Map of fences indexed by pathname of fence. Several different use
// cases; for example, used by fssrv to keep track of all fences seen,
// and used by a sess to keep track of fences for that sess.
//

type entry struct {
	qid  np.Tqid
	nref int
}

type FenceTable struct {
	sync.Mutex
	fences map[string]*entry
}

func MakeFenceTable() *FenceTable {
	fm := &FenceTable{}
	fm.fences = make(map[string]*entry)
	return fm
}

func (fm *FenceTable) Register(req np.Tfence) error {
	fm.Lock()
	defer fm.Unlock()

	fn := np.Join(req.Wnames)
	if f, ok := fm.fences[fn]; ok {
		if stale := f.qid.IsStale(req.Qid); stale {
			return fmt.Errorf("stale %v", fn)
		}
		fm.fences[fn].qid = req.Qid
		if req.New {
			f.nref += 1
		}
		return nil
	}
	fm.fences[fn] = &entry{req.Qid, 1}
	return nil
}

func (fm *FenceTable) Check(myf *Fence) error {
	fm.Lock()
	defer fm.Unlock()

	if e, ok := fm.fences[myf.Path]; ok {
		if e.qid.IsStale(myf.Qid) {
			log.Printf("stale %v\n", myf.Path)
			return fmt.Errorf("stale %v", myf.Path)
		}
		return nil
	}
	return fmt.Errorf("unknown fence %v\n", myf.Path)
}

func (fm *FenceTable) Unregister(path []string) error {
	fm.Lock()
	defer fm.Unlock()

	fn := np.Join(path)
	if e, ok := fm.fences[fn]; !ok {
		return fmt.Errorf("unknown fence", fn)
	} else {
		e.nref -= 1
		if e.nref == 0 {
			delete(fm.fences, fn)
		}
	}
	return nil
}

func (fm *FenceTable) Fences() []*Fence {
	fm.Lock()
	defer fm.Unlock()

	fences := make([]*Fence, 0, len(fm.fences))
	for f, e := range fm.fences {
		fences = append(fences, MakeFence(f, e.qid))
	}
	return fences
}

// Adding without reference counting, because there is only 1.  Used
// by sess.
func (fm *FenceTable) Add(path string, qid np.Tqid) error {
	fm.Lock()
	defer fm.Unlock()

	if _, ok := fm.fences[path]; ok {
		return fmt.Errorf("fence already present %v", path)
	}
	fm.fences[path] = &entry{qid, 1}
	return nil
}

// Adding without reference counting, because there is only 1.  Used
// by sess.
func (fm *FenceTable) Update(path string, qid np.Tqid) error {
	fm.Lock()
	defer fm.Unlock()

	if _, ok := fm.fences[path]; !ok {
		return fmt.Errorf("fence not present %v", path)
	}
	fm.fences[path] = &entry{qid, 1}
	return nil
}

// Deleting without reference counting.
func (fm *FenceTable) Del(path string) error {
	fm.Lock()
	defer fm.Unlock()

	if _, ok := fm.fences[path]; ok {
		delete(fm.fences, path)
		return nil
	}
	return fmt.Errorf("fence not present %v", path)
}

func (fm *FenceTable) Present(path string) bool {
	fm.Lock()
	defer fm.Unlock()

	_, ok := fm.fences[path]
	return ok
}
