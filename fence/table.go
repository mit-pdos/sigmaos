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
	fences map[np.Tfenceid]*entry
}

func MakeFenceTable() *FenceTable {
	fm := &FenceTable{}
	fm.fences = make(map[np.Tfenceid]*entry)
	return fm
}

// Make fenceid for the path so that we have unique name for the
// fence.  XXX when to delete?
func (fm *FenceTable) MkFence(path []string) np.Tfenceid {
	fm.Lock()
	defer fm.Unlock()

	fn := np.Join(path)
	idf := np.Tfenceid{fn, 0}
	return idf
}

// If no fence exists for this fn, register it.  If the fence is for
// the same incarnation of the file, but newer, update the fence.  If
// requests updates the fence with new qid, the last qid should match
// currently registered.
func (fm *FenceTable) Register(req np.Tfence) error {
	fm.Lock()
	defer fm.Unlock()

	idf := req.Fence
	if f, ok := fm.fences[idf]; ok {
		log.Printf("Register: fence %v %v\n", idf, f)
		if req.Last == req.Qid { // new registration?
			if fresh := f.qid.IsFresh(req.Qid); !fresh {
				return fmt.Errorf("stale %v", idf)
			}
			fm.fences[idf].qid = req.Qid
			f.nref += 1
		} else { // update existing registration
			// Too be able to update, req.Last must be
			// fresh. If so, then replace the old qid with
			// the new one, which may be a new incarnation
			// of the fence.
			if fresh := f.qid.IsFresh(req.Last); !fresh {
				return fmt.Errorf("stale %v", idf)
			}
			fm.fences[idf].qid = req.Qid
		}
		return nil
	}
	fm.fences[idf] = &entry{req.Qid, 1}
	return nil
}

// For a fenced path, update qid unconditionally (e.g., when file
// creator succeeds, and has the file lock).  The previous creator is
// now fenced off.  XXX nref
func (fm *FenceTable) UpdateFence(path []string, qid np.Tqid) {
	fm.Lock()
	defer fm.Unlock()

	idf := np.Tfenceid{np.Join(path), 0}
	if f, ok := fm.fences[idf]; ok {
		log.Printf("UpdateFence: update fenceid %v/%v %v to %v\n", path, idf, f, qid)
		fm.fences[idf].qid = qid
	} else {
		log.Printf("UpdateFence: no fenceid %v/%v\n", path, idf)
	}
}

func (fm *FenceTable) Check(myf *Fence) error {
	fm.Lock()
	defer fm.Unlock()

	if e, ok := fm.fences[myf.Fence]; ok {
		if !e.qid.IsFresh(myf.Qid) {
			log.Printf("stale %v\n", myf.Fence)
			return fmt.Errorf("stale %v", myf.Fence)
		}
		return nil
	}
	return fmt.Errorf("unknown fence %v\n", myf.Fence)
}

func (fm *FenceTable) Unregister(idf np.Tfenceid) error {
	fm.Lock()
	defer fm.Unlock()

	if e, ok := fm.fences[idf]; !ok {
		return fmt.Errorf("unknown fence %v", idf)
	} else {
		e.nref -= 1
		if e.nref == 0 {
			log.Printf("Unregister: remove %v\n", idf)
			delete(fm.fences, idf)
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
func (fm *FenceTable) Add(idf np.Tfenceid, qid np.Tqid) error {
	fm.Lock()
	defer fm.Unlock()

	if _, ok := fm.fences[idf]; ok {
		return fmt.Errorf("fence already present %v", idf)
	}
	fm.fences[idf] = &entry{qid, 1}
	return nil
}

// Adding without reference counting, because there is only 1.  Used
// by sess.
func (fm *FenceTable) Update(idf np.Tfenceid, qid np.Tqid) error {
	fm.Lock()
	defer fm.Unlock()

	if _, ok := fm.fences[idf]; !ok {
		return fmt.Errorf("fence not present %v", idf)
	}
	fm.fences[idf] = &entry{qid, 1}
	return nil
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
