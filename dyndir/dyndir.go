// Package dyndir watches a changing directory and keeps a local copy
// with entries of type E chosen by the caller (e.g., rpcclnt's as in
// [unionrpcclnt]).  dyndir updates the entries as file as
// created/removed in the watched directory.

package dyndir

import (
	"sync"
	"sync/atomic"

	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

type NewEntryF[E any] func(string) (E, error)

type DynDir[E any] struct {
	*fslib.FsLib
	sync.Mutex
	dir       *sorteddir.SortedDir[string, E]
	watching  bool
	done      uint32
	Path      string
	LSelector db.Tselector
	ESelector db.Tselector
	newEntry  NewEntryF[E]
}

func NewDynDir[E any](fsl *fslib.FsLib, path string, newEntry NewEntryF[E], lSelector db.Tselector, ESelector db.Tselector) *DynDir[E] {
	dd := &DynDir[E]{
		FsLib:     fsl,
		Path:      path,
		dir:       sorteddir.NewSortedDir[string, E](),
		LSelector: lSelector,
		ESelector: ESelector,
		newEntry:  newEntry,
	}
	return dd
}

func (dd *DynDir[E]) Nentry() (int, error) {
	es, err := dd.GetEntries()
	if err != nil {
		return 0, err
	}
	return len(es), nil
}

func (dd *DynDir[E]) GetEntry(n string) (E, bool) {
	var ok bool
	var e E
	db.DPrintf(dd.LSelector, "GetEntry for %v", n)
	defer func(e *E, ok *bool) {
		db.DPrintf(dd.LSelector, "Done GetEntry for %v e %v ok %t", n, *e, *ok)
	}(&e, &ok)
	e, ok = dd.dir.Lookup(n)
	return e, ok
}

func (dd *DynDir[E]) GetEntryAlloc(n string) (E, error) {
	db.DPrintf(dd.LSelector, "GetEntryAlloc for %v", n)
	defer db.DPrintf(dd.LSelector, "Done GetEntryAlloc for %v", n)

	dd.Lock()
	defer dd.Unlock()

	e, ok := dd.dir.Lookup(n)
	if !ok {
		e1, err := dd.newEntry(n)
		if err != nil {
			return e1, err
		}
		e = e1
		dd.dir.Insert(n, e)
	}
	return e, nil
}

func (dd *DynDir[E]) UpdateEntries(force bool) {
	db.DPrintf(dd.LSelector, "UpdateSrvs")
	defer db.DPrintf(dd.LSelector, "Done UpdateSrvs")

	dd.Lock()
	defer dd.Unlock()

	if !dd.watching {
		go dd.watchDir()
		dd.watching = true
	}

	// If the caller is not forcing an update, and the list of ents
	// has already been populated, do nothing and return.
	if !force && dd.dir.Len() > 0 {
		db.DPrintf(dd.LSelector, "No need to update srv list")
		return
	}

	ents, err := dd.GetEntries()
	if err != nil {
		db.DPrintf(db.ALWAYS, "GetEntries %v", err)
		return
	}
	dd.updateEntriesL(ents)
}

// Caller must hold dd mutex
func (dd *DynDir[E]) updateEntriesL(ents []string) error {
	db.DPrintf(dd.LSelector, "Update ents %v in %v", ents, dd.dir)
	entsMap := map[string]bool{}
	for _, n := range ents {
		entsMap[n] = true
		if _, ok := dd.dir.Lookup(n); !ok {
			e, err := dd.newEntry(n)
			if err != nil {
				return err
			}
			dd.dir.Insert(n, e)
		}
	}
	for _, n := range dd.dir.Keys(0) {
		if !entsMap[n] {
			dd.dir.Delete(n)
		}
	}
	db.DPrintf(dd.LSelector, "Update ents %v done %v", ents, dd.dir)
	return nil
}

func (dd *DynDir[E]) Random() (string, bool) {
	var n string
	var ok bool
	db.DPrintf(dd.LSelector, "Random")
	defer func(n *string) {
		db.DPrintf(dd.LSelector, "Done Random %v %t", *n, ok)
	}(&n)
	n, ok = dd.dir.Random()
	return n, ok
}

func (dd *DynDir[E]) RoundRobin() (string, bool) {
	var n string
	var ok bool
	db.DPrintf(dd.LSelector, "RoundRobin")
	defer func(n *string) {
		db.DPrintf(dd.LSelector, "Done RoundRobin %v %t", *n, ok)
	}(&n)

	n, ok = dd.dir.RoundRobin()
	return n, ok
}

func (dd *DynDir[E]) Remove(name string) bool {
	db.DPrintf(dd.LSelector, "UnregisterSrv %v", name)
	defer db.DPrintf(dd.LSelector, "Done UnregisterSrv")
	return dd.dir.Delete(name)
}

// Read directory from server. The caller may hold the dd mutex
func (dd *DynDir[E]) GetEntries() ([]string, error) {
	sts, err := dd.GetDir(dd.Path)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}

func (dd *DynDir[E]) StopWatching() {
	atomic.StoreUint32(&dd.done, 1)
}

// Monitor for changes to the set of entries listed in the directory.
func (dd *DynDir[E]) watchDir() {
	for atomic.LoadUint32(&dd.done) != 1 {
		var ents []string
		err := dd.ReadDirWatch(dd.Path, func(sts []*sp.Stat) bool {
			ents = sp.Names(sts)

			// If the lengths don't match, the dir has changed. Return
			// false to stop watching the dir and return into watchDir.
			if len(ents) != dd.dir.Len() {
				return false
			}

			for _, n := range ents {
				_, ok := dd.dir.Lookup(n)
				if !ok {
					// If a name is not present in dir, then there has
					// been a change to the dir; stop watch.
					db.DPrintf(dd.LSelector, "Lookup %v not present", n)
					return false
				}
			}
			return true
		})
		if err != nil {
			db.DPrintf(dd.ESelector, "Error ReadDirWatch watchDir[%v]: %v", dd.Path, err)
		}
		db.DPrintf(dd.LSelector, "watchDir new ents %v", ents)
		dd.Lock()
		dd.updateEntriesL(ents)
		dd.Unlock()
	}
}
