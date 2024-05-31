// Package dyndir watches a changing directory and keeps a local copy
// with entries of type E chosen by the caller (e.g., rpcclnt's as in
// [unionrpcclnt]).  dyndir updates the entries as file as
// created/removed in the watched directory.

package dyndir

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

type DynDir[E any] struct {
	*fslib.FsLib
	sync.Mutex
	dir       *sortedmap.SortedMap[string, E]
	watching  bool
	done      uint32
	Path      string
	LSelector db.Tselector
	ESelector db.Tselector
	newEntry  NewEntryF[E]
	err       error
}

func NewDynDir[E any](fsl *fslib.FsLib, path string, newEntry NewEntryF[E], lSelector db.Tselector, ESelector db.Tselector) *DynDir[E] {
	dd := &DynDir[E]{
		FsLib:     fsl,
		Path:      path,
		dir:       sortedmap.NewSortedMap[string, E](),
		LSelector: lSelector,
		ESelector: ESelector,
		newEntry:  newEntry,
	}
	return dd
}

func (dd *DynDir[E]) Nentry() (int, error) {
	if err := dd.watchEntries(false); err != nil {
		return 0, err
	}
	return dd.dir.Len(), nil
}

func (dd *DynDir[E]) GetEntries() ([]string, error) {
	if err := dd.watchEntries(false); err != nil {
		return nil, err
	}
	return dd.dir.Keys(0), nil
}

func (dd *DynDir[E]) GetEntry(n string) (E, error) {
	var ok bool
	var e E
	db.DPrintf(dd.LSelector, "GetEntry for %v", n)
	defer func(e *E, ok *bool) {
		db.DPrintf(dd.LSelector, "Done GetEntry for %v e %v ok %t", n, *e, *ok)
	}(&e, &ok)
	if err := dd.watchEntries(false); err != nil {
		return e, err
	}
	e, ok = dd.dir.Lookup(n)
	if !ok {
		return e, serr.NewErr(serr.TErrNotfound, n)
	}
	return e, nil
}

func (dd *DynDir[E]) GetEntryAlloc(n string) (E, error) {
	db.DPrintf(dd.LSelector, "GetEntryAlloc for %v", n)
	defer db.DPrintf(dd.LSelector, "Done GetEntryAlloc for %v", n)

	if err := dd.watchEntries(false); err != nil {
		var e E
		return e, err
	}

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

func (dd *DynDir[E]) watchEntries(force bool) error {
	if force {
		db.DPrintf(dd.LSelector, "watchEntries")
		defer db.DPrintf(dd.LSelector, "Done watchEntries")
	}

	dd.Lock()
	defer dd.Unlock()

	if dd.err != nil {
		db.DPrintf(dd.LSelector, "watchEntries %v", dd.err)
		return dd.err
	}

	if !dd.watching {
		go dd.watchDir()
		dd.watching = true
	}

	// If the caller is not forcing an update, and the list of ents
	// has already been populated, do nothing and return.
	if !force && dd.dir.Len() > 0 {
		return nil
	}

	ents, err := dd.getEntries()
	if err != nil {
		db.DPrintf(db.ALWAYS, "getEntries %v", err)
		return err
	}
	return dd.updateEntriesL(ents)
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

func (dd *DynDir[E]) Random() (string, error) {
	var n string
	var ok bool

	db.DPrintf(dd.LSelector, "Random")

	if err := dd.watchEntries(false); err != nil {
		return "", err
	}
	defer func(n *string) {
		db.DPrintf(dd.LSelector, "Done Random %v %t", *n, ok)
	}(&n)
	n, ok = dd.dir.Random()
	if !ok {
		return "", serr.NewErr(serr.TErrNotfound, "no random entry")
	}
	return n, nil
}

func (dd *DynDir[E]) RoundRobin() (string, error) {
	var n string
	var ok bool

	db.DPrintf(dd.LSelector, "RoundRobin")

	if err := dd.watchEntries(false); err != nil {
		return "", err
	}

	defer func(n *string) {
		db.DPrintf(dd.LSelector, "Done RoundRobin %v %t", *n, ok)
	}(&n)

	n, ok = dd.dir.RoundRobin()
	if !ok {
		return "", serr.NewErr(serr.TErrNotfound, "no next entry")
	}
	return n, nil
}

func (dd *DynDir[E]) Remove(name string) bool {
	db.DPrintf(dd.LSelector, "UnregisterSrv %v", name)
	defer db.DPrintf(dd.LSelector, "Done UnregisterSrv")
	return dd.dir.Delete(name)
}

// Read directory from server and return unique files. The caller may
// hold the dd mutex
func (dd *DynDir[E]) getEntries() ([]string, error) {
	s := time.Now()
	defer db.DPrintf(db.SPAWN_LAT, "[%v] getEntries %v", dd.LSelector, time.Since(s))

	fw := fslib.NewFileWatcher(dd.FsLib, dd.Path)
	fns, err := fw.GetUniqueEntries()
	if err != nil {
		return nil, err
	}
	return fns, nil
}

func (dd *DynDir[E]) StopWatching() {
	atomic.StoreUint32(&dd.done, 1)
}

// Monitor for changes to the directory and update the cached one
func (dd *DynDir[E]) watchDir() {
	retry := false
	for atomic.LoadUint32(&dd.done) != 1 {
		fw := fslib.NewFileWatcher(dd.FsLib, dd.Path)
		ents, ok, err := fw.WatchUniqueEntries(dd.dir.Keys(0))
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
				db.DPrintf(dd.ESelector, "watchDir[%v]: %t %v retry watching", dd.Path, ok, err)
				continue
			} else { // give up
				db.DPrintf(dd.ESelector, "watchDir[%v]: %t %v stop watching", dd.Path, ok, err)
				dd.err = err
				dd.watching = false
				return
			}
		}
		db.DPrintf(dd.LSelector, "watchDir new ents %v", ents)
		dd.Lock()
		dd.updateEntriesL(ents)
		dd.Unlock()
	}
}
