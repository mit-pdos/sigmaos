// Package dircache watches a changing directory and keeps a local copy
// with entries of type E chosen by the caller (e.g., rpcclnt's as in
// [rpcdirclnt]).  dircache updates the entries as file as
// created/removed in the watched directory.

package dircache

import (
	"sync"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sortedmap"
)

type NewEntryF[E any] func(string) (E, error)

type DirCache[E any] struct {
	*fslib.FsLib
	sync.Mutex
	dir       *sortedmap.SortedMap[string, E]
	watching  bool
	done      atomic.Uint64
	Path      string
	LSelector db.Tselector
	ESelector db.Tselector
	newEntry  NewEntryF[E]
	err       error
}

func NewDirCache[E any](fsl *fslib.FsLib, path string, newEntry NewEntryF[E], lSelector db.Tselector, ESelector db.Tselector) *DirCache[E] {
	dd := &DirCache[E]{
		FsLib:     fsl,
		Path:      path,
		dir:       sortedmap.NewSortedMap[string, E](),
		LSelector: lSelector,
		ESelector: ESelector,
		newEntry:  newEntry,
	}
	return dd
}

func (dc *DirCache[E]) Nentry() (int, error) {
	if err := dc.watchEntries(false); err != nil {
		return 0, err
	}
	return dc.dir.Len(), nil
}

func (dc *DirCache[E]) GetEntries() ([]string, error) {
	if err := dc.watchEntries(false); err != nil {
		return nil, err
	}
	return dc.dir.Keys(0), nil
}

func (dc *DirCache[E]) GetEntry(n string) (E, error) {
	var ok bool
	var e E
	db.DPrintf(dc.LSelector, "GetEntry for %v", n)
	defer func(e *E, ok *bool) {
		db.DPrintf(dc.LSelector, "Done GetEntry for %v e %v ok %t", n, *e, *ok)
	}(&e, &ok)
	if err := dc.watchEntries(false); err != nil {
		return e, err
	}
	e, ok = dc.dir.Lookup(n)
	if !ok {
		return e, serr.NewErr(serr.TErrNotfound, n)
	}
	return e, nil
}

func (dc *DirCache[E]) GetEntryAlloc(n string) (E, error) {
	db.DPrintf(dc.LSelector, "GetEntryAlloc for %v", n)
	defer db.DPrintf(dc.LSelector, "Done GetEntryAlloc for %v", n)

	if err := dc.watchEntries(false); err != nil {
		var e E
		return e, err
	}

	dc.Lock()
	defer dc.Unlock()
	e, ok := dc.dir.Lookup(n)
	if !ok {
		e1, err := dc.newEntry(n)
		if err != nil {
			return e1, err
		}
		e = e1
		dc.dir.Insert(n, e)
	}
	return e, nil
}

func (dc *DirCache[E]) watchEntries(force bool) error {
	if force {
		db.DPrintf(dc.LSelector, "watchEntries")
		defer db.DPrintf(dc.LSelector, "Done watchEntries")
	}

	dc.Lock()
	defer dc.Unlock()

	if dc.err != nil {
		db.DPrintf(dc.LSelector, "watchEntries %v", dc.err)
		return dc.err
	}

	if !dc.watching {
		go dc.watchDir()
		dc.watching = true
	}

	// If the caller is not forcing an update, and the list of ents
	// has already been populated, do nothing and return.
	if !force && dc.dir.Len() > 0 {
		return nil
	}

	ents, err := dc.getEntries()
	if err != nil {
		db.DPrintf(db.ALWAYS, "getEntries %v", err)
		return err
	}
	return dc.updateEntriesL(ents)
}

// Caller must hold dd mutex
func (dc *DirCache[E]) updateEntriesL(ents []string) error {
	db.DPrintf(dc.LSelector, "Update ents %v in %v", ents, dc.dir)
	entsMap := map[string]bool{}
	for _, n := range ents {
		entsMap[n] = true
		if _, ok := dc.dir.Lookup(n); !ok {
			e, err := dc.newEntry(n)
			if err != nil {
				return err
			}
			dc.dir.Insert(n, e)
		}
	}
	for _, n := range dc.dir.Keys(0) {
		if !entsMap[n] {
			dc.dir.Delete(n)
		}
	}
	db.DPrintf(dc.LSelector, "Update ents %v done %v", ents, dc.dir)
	return nil
}

func (dc *DirCache[E]) Random() (string, error) {
	var n string
	var ok bool

	db.DPrintf(dc.LSelector, "Random")

	if err := dc.watchEntries(false); err != nil {
		return "", err
	}
	defer func(n *string) {
		db.DPrintf(dc.LSelector, "Done Random %v %t", *n, ok)
	}(&n)
	n, ok = dc.dir.Random()
	if !ok {
		return "", serr.NewErr(serr.TErrNotfound, "no random entry")
	}
	return n, nil
}

func (dc *DirCache[E]) RoundRobin() (string, error) {
	var n string
	var ok bool

	db.DPrintf(dc.LSelector, "RoundRobin")

	if err := dc.watchEntries(false); err != nil {
		return "", err
	}

	defer func(n *string) {
		db.DPrintf(dc.LSelector, "Done RoundRobin %v %t", *n, ok)
	}(&n)

	n, ok = dc.dir.RoundRobin()
	if !ok {
		return "", serr.NewErr(serr.TErrNotfound, "no next entry")
	}
	return n, nil
}

func (dc *DirCache[E]) RemoveEntry(name string) bool {
	db.DPrintf(dc.LSelector, "RemoveEntry %v", name)
	ok := dc.dir.Delete(name)
	db.DPrintf(dc.LSelector, "Done Remove entry %v %v", ok, dc.dir)
	return ok
}

// Read directory from server and return unique files. The caller may
// hold the dd mutex
func (dc *DirCache[E]) getEntries() ([]string, error) {
	s := time.Now()
	defer db.DPrintf(db.SPAWN_LAT, "getEntries %v", time.Since(s))

	dr := fslib.NewDirReader(dc.FsLib, dc.Path)
	fns, err := dr.GetUniqueEntries()
	if err != nil {
		db.DPrintf(dc.ESelector, "getEntries %v err", err)
		return nil, err
	}
	db.DPrintf(dc.LSelector, "getEntries %v", fns)
	return fns, nil
}

func (dc *DirCache[E]) StopWatching() {
	dc.done.Add(1)
}

// Monitor for changes to the directory and update the cached one
func (dc *DirCache[E]) watchDir() {
	retry := false
	for dc.done.Load() == 0 {
		dr := fslib.NewDirReader(dc.FsLib, dc.Path)
		ents, ok, err := dr.WatchUniqueEntries(dc.dir.Keys(0))
		if ok { // reset retry?
			retry = false
		}
		if err != nil {
			if serr.IsErrorUnreachable(err) && !retry {
				time.Sleep(sp.PATHCLNT_TIMEOUT * time.Millisecond)
				// try again but remember we are already tried reading ReadDir
				if !ok {
					retry = true
				}
				db.DPrintf(dc.ESelector, "watchDir[%v]: %t %v retry watching", dc.Path, ok, err)
				continue
			} else { // give up
				db.DPrintf(dc.ESelector, "watchDir[%v]: %t %v stop watching", dc.Path, ok, err)
				dc.err = err
				dc.watching = false
				return
			}
		}
		db.DPrintf(dc.LSelector, "watchDir new ents %v", ents)
		dc.Lock()
		dc.updateEntriesL(ents)
		dc.Unlock()
	}
}
