package fence

import (
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

//
// Map of fences indexed by pathname of fence at server. Map has
// several different use cases: for example, used by fssrv to keep
// track of all fences seen, and used by a sess to keep track of
// fences for that sess.
//

type entry struct {
	fence np.Tfence
	nref  int // XXX keep track of all sess
}

type FenceTable struct {
	sync.Mutex
	fences map[np.Tfenceid]*entry
}

func MakeFenceTable() *FenceTable {
	fm := &FenceTable{}
	fm.fences = make(map[np.Tfenceid]*entry)
	return fm
}

// Server that holds the fence file uses MkFence to make a fence.  XXX
// Can we ever delete the fence?
func (fm *FenceTable) MkFence(path []string) np.Tfence {
	fm.Lock()
	defer fm.Unlock()

	fn := np.Join(path)
	idf := np.Tfenceid{fn, 0}

	if e, ok := fm.fences[idf]; ok {
		return e.fence
	}

	fence := np.Tfence{idf, 1}
	fm.fences[idf] = &entry{fence, 1}

	return fence
}

// A new acquisition of a file or a modification of a file that may
// have a fence associated with it. If so, increase seqno of fence.
func (fm *FenceTable) UpdateFence(path []string) {
	fm.Lock()
	defer fm.Unlock()

	idf := np.Tfenceid{np.Join(path), 0}
	if e, ok := fm.fences[idf]; ok {
		e.fence.Seqno += 1
		log.Printf("UpdateFence: updated seqno %v %v\n", idf, e)
	} else {
		// log.Printf("UpdateFence: no fenceid %v/%v\n", path, idf)
	}
}

// Client registers a fence at this server. The registration may a new
// or an update. If no fence exists for this fence id, register it.
// If the fence exists but newer, update the fence.  If this is a new
// registration increase refcnt; unregister will decrement it.
func (fm *FenceTable) Register(req np.Tregfence) error {
	fm.Lock()
	defer fm.Unlock()

	idf := req.Fence.FenceId
	if e, ok := fm.fences[idf]; ok {
		log.Printf("%v: Register: fence %v %v\n", db.GetName(), idf, req)
		if req.Fence.Seqno < e.fence.Seqno {
			return fmt.Errorf("stale %v", idf)
		}
		fm.fences[idf].fence = req.Fence
		if req.New {
			e.nref += 1
		}
		return nil
	}
	fm.fences[idf] = &entry{req.Fence, 1}
	return nil
}

func (fm *FenceTable) Unregister(fence np.Tfence) error {
	fm.Lock()
	defer fm.Unlock()

	if e, ok := fm.fences[fence.FenceId]; !ok {
		return fmt.Errorf("unknown fence %v", fence.FenceId)
	} else {
		//if fence.Seqno < e.fence.Seqno {
		//	log.Printf("Unregister: stale %v\n", fence)
		//	return fmt.Errorf("stale fence %v", fence)
		//}
		e.nref -= 1
		if e.nref == 0 {
			log.Printf("Unregister: remove %v\n", fence)
			delete(fm.fences, fence.FenceId)
		}
	}
	return nil
}

// Check if supplied fence is recent.
func (fm *FenceTable) IsRecent(fence np.Tfence) error {
	fm.Lock()
	defer fm.Unlock()

	if e, ok := fm.fences[fence.FenceId]; ok {
		if fence.Seqno < e.fence.Seqno {
			log.Printf("stale %v\n", fence)
			return fmt.Errorf("stale fence %v", fence)
		}
		return nil
	}
	return fmt.Errorf("unknown fence %v\n", fence.FenceId)
}

//
// Code below is used by a sesion and fsclnt to keep track of its
// fences.
//

func (fm *FenceTable) Fences() []np.Tfence {
	fm.Lock()
	defer fm.Unlock()

	fences := make([]np.Tfence, 0, len(fm.fences))
	for _, e := range fm.fences {
		fences = append(fences, e.fence)
	}
	return fences
}

// Insert without reference counting, because there is only 1.  Used
// by sess.
func (fm *FenceTable) Insert(f np.Tfence) {
	fm.Lock()
	defer fm.Unlock()

	if e, ok := fm.fences[f.FenceId]; ok {
		e.fence = f
	} else {
		fm.fences[f.FenceId] = &entry{f, 1}
	}
}

// Deleting without reference counting.
func (fm *FenceTable) Del(idf np.Tfenceid) error {
	fm.Lock()
	defer fm.Unlock()

	if _, ok := fm.fences[idf]; ok {
		delete(fm.fences, idf)
		return nil
	}
	return fmt.Errorf("fence not present %v", idf)
}

func (fm *FenceTable) Present(idf np.Tfenceid) bool {
	fm.Lock()
	defer fm.Unlock()

	_, ok := fm.fences[idf]
	return ok
}
