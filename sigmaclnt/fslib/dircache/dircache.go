// Package dircache watches a changing directory and keeps a local copy
// with entries of type E chosen by the caller (e.g., rpcclnt's as in
// [rpcdirclnt]).  dircache updates the entries as files are
// created/removed in the watched directory.
package dircache

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sigmaclnt/fslib/dirwatcher"
	sp "sigmaos/sigmap"
	protsrv_proto "sigmaos/spproto/srv/proto"
	"sigmaos/util/sortedmapv1"
	"sigmaos/util/spstats"
)

type NewValF[E any] func(sp.Tsigmapath) (E, error)

type InitState uint64

const (
	NotInitialized InitState = iota
	InProgress
	Finished
)

type DirCacheStats struct {
	Ninvalidate spstats.Tcounter
}

type DirCache[E any] struct {
	*fslib.FsLib
	sync.RWMutex
	hasEntries    *sync.Cond
	dir           *sortedmapv1.SortedMap[sp.Tsigmapath, E]
	isDone        atomic.Uint64
	initState     InitState
	initCond      sync.Cond
	version       uint64
	Path          sp.Tsigmapath
	LSelector     db.Tselector
	ESelector     db.Tselector
	newVal        NewValF[E]
	prefixFilters []sp.Tsigmapath
	err           error
	ch            chan string
	stat          DirCacheStats
}

func NewDirCache[E any](fsl *fslib.FsLib, path sp.Tsigmapath, newVal NewValF[E], ch chan string, lSelector db.Tselector, ESelector db.Tselector) *DirCache[E] {
	return NewDirCacheFilter(fsl, path, newVal, ch, lSelector, ESelector, nil)
}

// filter entries starting with prefix
func NewDirCacheFilter[E any](fsl *fslib.FsLib, path sp.Tsigmapath, newVal NewValF[E], ch chan string, LSelector db.Tselector, ESelector db.Tselector, prefixes []sp.Tsigmapath) *DirCache[E] {
	dc := &DirCache[E]{
		FsLib:         fsl,
		Path:          path,
		dir:           sortedmapv1.NewSortedMap[sp.Tsigmapath, E](),
		initCond:      *sync.NewCond(&sync.Mutex{}),
		LSelector:     LSelector,
		ESelector:     ESelector,
		newVal:        newVal,
		prefixFilters: prefixes,
		ch:            ch,
	}
	db.DPrintf(dc.LSelector, "NewDirCacheFilter: dir %v", path)
	dc.hasEntries = sync.NewCond(&dc.RWMutex)
	return dc
}

func (dc *DirCache[E]) Init() {
	dc.initCond.L.Lock()
	defer dc.initCond.L.Unlock()

	if dc.initState == NotInitialized {
		dc.initState = InProgress
		dw := dc.readDirAndWatch()
		if dw == nil {
			return
		}
		go dc.watchDir(dw)
		go dc.watchdog()
		dc.initState = Finished
	} else if dc.initState == InProgress {
		for dc.initState == InProgress && dc.checkErr() == nil {
			dc.initCond.Wait()
		}
	}
}

func (dc *DirCache[E]) Stats() *spstats.TcounterSnapshot {
	stro := spstats.NewTcounterSnapshot()
	stro.FillCounters(&dc.stat)
	return stro
}

func (dc *DirCache[E]) isKeyValid(k sp.Tsigmapath) bool {
	for _, prefix := range dc.prefixFilters {
		if strings.HasPrefix(k, prefix) {
			return false
		}
	}
	return true
}

func (dc *DirCache[E]) readDirAndWatch() *dirwatcher.DirWatcher {
	var initEnts []string
	var dw *dirwatcher.DirWatcher

	for dc.isDone.Load() == 0 {
		var err error
		initEnts, dw, err = dirwatcher.NewDirWatcherWithRead(dc.FsLib, dc.Path)
		if err != nil {
			if serr.IsErrorSession(err) {
				continue
			} else {
				dc.err = err
				return nil
			}
		}

		break
	}

	dc.Lock()
	for _, ent := range initEnts {
		if !dc.isKeyValid(ent) {
			continue
		}

		dc.dir.InsertKey(ent)
		if dc.ch != nil {
			go func(ent string) {
				dc.ch <- ent
			}(ent)
		}
	}
	dc.version += 1
	if dc.dir.Len() > 0 {
		dc.hasEntries.Broadcast()
	}
	dc.Unlock()

	if db.WillBePrinted(dc.LSelector) {
		db.DPrintf(dc.LSelector, "readDirAndWatch: %v", dc.dir.Keys())
	}

	return dw
}

// Monitor for changes to the directory and update the cached one
func (dc *DirCache[E]) watchDir(dw *dirwatcher.DirWatcher) {
	for event := range dw.Events() {
		if !dc.isKeyValid(event.File) {
			continue
		}

		dc.Lock()
		var madeChange bool
		switch event.Type {
		case protsrv_proto.WatchEventType_CREATE:
			db.DPrintf(dc.LSelector, "watchDir: create %v", event.File)
			madeChange = dc.dir.InsertKey(event.File)
			if madeChange && dc.ch != nil {
				go func(ent string) {
					dc.ch <- ent
				}(event.File)
			}
		case protsrv_proto.WatchEventType_REMOVE:
			db.DPrintf(dc.LSelector, "watchDir: remove %v", event.File)
			madeChange = dc.dir.Delete(event.File)
		}
		if madeChange {
			dc.hasEntries.Broadcast()
			dc.version += 1
		}
		dc.Unlock()
		if dc.isDone.Load() != 0 {
			break
		}
	}

	// if watch got interrupted not due to a dircache error, reset watcher
	if dc.isDone.Load() == 0 && dc.err == nil {
		db.DPrintf(dc.LSelector, "watchDir: restarting watcher")
		dw.Close()

		dc.initCond.L.Lock()
		dc.initState = NotInitialized
		dc.initCond.L.Unlock()

		dc.Init()
	} else {
		dw.Close()
	}
}

// watchdog thread that wakes up waiters periodically
func (dc *DirCache[E]) watchdog() {
	for dc.isDone.Load() == 0 {
		time.Sleep(fsetcd.LeaseTTL * time.Second)
		db.DPrintf(dc.LSelector, "watchdog: broadcast")
		dc.hasEntries.Broadcast()
	}
}

func (dc *DirCache[E]) StopWatching() {
	dc.isDone.Add(1)
}

func (dc *DirCache[E]) Nentry() (int, error) {
	dc.Init()
	if err := dc.checkErr(); err != nil {
		return 0, err
	}
	return dc.dir.Len(), nil
}

func (dc *DirCache[E]) GetEntries() ([]sp.Tsigmapath, error) {
	dc.Init()
	if err := dc.checkErr(); err != nil {
		return nil, err
	}
	return dc.dir.Keys(), nil
}

func (dc *DirCache[E]) getEntry(n sp.Tsigmapath, alloc bool) (E, uint64, error) {
	db.DPrintf(dc.LSelector, "GetEntry for %v", n)
	dc.Init()

	if err := dc.checkErr(); err != nil {
		var e E
		db.DPrintf(dc.LSelector, "Done GetEntry for %v err %v", n, err)
		return e, 0, err
	}
	var err error

	dc.Lock()
	version := dc.version
	kok, e, vok := dc.dir.LookupKeyVal(n)
	dc.Unlock()

	if !kok {
		db.DPrintf(dc.LSelector, "Done GetEntry for %v kok %v", n, kok)
		return e, version, serr.NewErr(serr.TErrNotfound, n)
	}
	if !vok && alloc {
		e, err = dc.allocVal(n)
	}
	db.DPrintf(dc.LSelector, "Done GetEntry for %v e %v err %v", n, e, err)
	return e, version, err
}

func (dc *DirCache[E]) GetEntry(n sp.Tsigmapath) (E, error) {
	ent, _, err := dc.getEntry(n, true)
	return ent, err
}

func (dc *DirCache[E]) InvalidateEntry(name sp.Tsigmapath) bool {
	dc.Init()
	db.DPrintf(dc.LSelector, "InvalidateEntry %v", name)
	spstats.Inc(&dc.stat.Ninvalidate, 1)
	ok := dc.dir.Delete(name)
	db.DPrintf(dc.LSelector, "Done invalidate entry %v %v", ok, dc.dir)
	return ok
}

func (dc *DirCache[E]) allocVal(n sp.Tsigmapath) (E, error) {
	dc.RLock()
	defer dc.RUnlock()

	db.DPrintf(dc.LSelector, "GetEntryAlloc for %v", n)
	defer db.DPrintf(dc.LSelector, "Done GetEntryAlloc for %v", n)

	_, e, vok := dc.dir.LookupKeyVal(n)
	if !vok {
		dc.RUnlock()
		dc.Lock()

		_, e, vok = dc.dir.LookupKeyVal(n)
		if !vok {
			e1, err := dc.newVal(n)
			if err != nil {
				dc.Unlock()
				dc.RLock()
				return e1, err
			}
			e = e1
			dc.dir.Insert(n, e)
		}

		dc.Unlock()
		dc.RLock()
	}
	return e, nil
}

func (dc *DirCache[E]) LookupEntry(n sp.Tsigmapath) error {
	_, err := dc.GetEntry(n)
	return err
}

func (dc *DirCache[E]) randomEntry() (sp.Tsigmapath, uint64, error) {
	var n sp.Tsigmapath
	var ok bool

	db.DPrintf(dc.LSelector, "Random")
	dc.Init()

	if err := dc.checkErr(); err != nil {
		return "", 0, err
	}
	defer func(n *sp.Tsigmapath) {
		db.DPrintf(dc.LSelector, "Done Random %v %t", *n, ok)
	}(&n)

	dc.Lock()
	version := dc.version
	n, ok = dc.dir.Random()
	dc.Unlock()

	if !ok {
		return "", version, serr.NewErr(serr.TErrNotfound, "no random entry")
	}
	return n, version, nil
}

func (dc *DirCache[E]) roundRobin() (sp.Tsigmapath, uint64, error) {
	var n sp.Tsigmapath
	var ok bool

	db.DPrintf(dc.LSelector, "RoundRobin")
	dc.Init()

	if err := dc.checkErr(); err != nil {
		return "", 0, err
	}

	defer func(n *sp.Tsigmapath) {
		db.DPrintf(dc.LSelector, "Done RoundRobin %v %t", *n, ok)
	}(&n)

	dc.Lock()
	version := dc.version
	n, ok = dc.dir.RoundRobin()
	dc.Unlock()

	if !ok {
		return "", version, serr.NewErr(serr.TErrNotfound, "no next entry")
	}
	return n, version, nil
}

func (dc *DirCache[E]) WaitTimedRandomEntry() (sp.Tsigmapath, error) {
	var entry sp.Tsigmapath
	err := dc.waitCond(func() (bool, uint64, error) {
		var version uint64
		var err error

		entry, version, err = dc.randomEntry()
		return serr.IsErrorNotfound(err), version, err
	}, true)

	return entry, err
}

func (dc *DirCache[E]) WaitTimedRoundRobin() (sp.Tsigmapath, error) {
	var entry sp.Tsigmapath
	err := dc.waitCond(func() (bool, uint64, error) {
		var version uint64
		var err error

		entry, version, err = dc.roundRobin()
		return serr.IsErrorNotfound(err), version, err
	}, true)

	return entry, err
}

func (dc *DirCache[E]) WaitEntryCreated(file sp.Tsigmapath) error {
	return dc.waitCond(func() (bool, uint64, error) {
		var version uint64
		var err error

		_, version, err = dc.getEntry(file, false)
		return serr.IsErrorNotfound(err), version, err
	}, false)
}

func (dc *DirCache[E]) WaitAllEntriesCreated(files []sp.Tsigmapath) error {
	added := make(map[sp.Tsigmapath]bool)
	return dc.waitCond(func() (bool, uint64, error) {
		var firstVersion = uint64(0)

		for _, file := range files {
			if _, ok := added[file]; ok {
				continue
			}

			_, version, err := dc.getEntry(file, false)
			if firstVersion == 0 {
				firstVersion = version
			}

			if err == nil {
				added[file] = true
				continue
			}

			if !serr.IsErrorNotfound(err) {
				return false, 0, err
			}
		}

		allAdded := len(added) == len(files)
		return !allAdded, firstVersion, nil
	}, false)
}

func (dc *DirCache[E]) WaitEntryRemoved(file sp.Tsigmapath) error {
	return dc.waitCond(func() (bool, uint64, error) {
		var version uint64
		var err error

		_, version, err = dc.getEntry(file, false)

		if serr.IsErrorNotfound(err) {
			return false, version, nil
		}

		return true, version, err
	}, false)
}

func (dc *DirCache[E]) WaitAllEntriesRemoved(files []sp.Tsigmapath) error {
	removed := make(map[sp.Tsigmapath]bool)
	return dc.waitCond(func() (bool, uint64, error) {
		var firstVersion = uint64(0)

		for _, file := range files {
			if _, ok := removed[file]; ok {
				continue
			}

			_, version, err := dc.getEntry(file, false)
			if firstVersion == 0 {
				firstVersion = version
			}

			if serr.IsErrorNotfound(err) {
				removed[file] = true
				continue
			}

			if err != nil {
				return false, 0, err
			}
		}

		allRemoved := len(removed) == len(files)
		return !allRemoved, firstVersion, nil
	}, false)
}

func (dc *DirCache[E]) WaitEntriesN(n int, timed bool) (int, error) {
	err := dc.waitCond(func() (bool, uint64, error) {
		dc.Lock()
		numFiles := dc.dir.Len()
		version := dc.version
		dc.Unlock()

		return numFiles < n, version, nil
	}, timed)

	return dc.dir.Len(), err
}

func (dc *DirCache[E]) WaitGetEntriesN(n int, timed bool) ([]sp.Tsigmapath, error) {
	if _, err := dc.WaitEntriesN(n, timed); err != nil {
		return nil, err
	}
	return dc.dir.Keys(), nil
}

func (dc *DirCache[E]) WaitEmpty() error {
	return dc.waitCond(func() (bool, uint64, error) {
		dc.Lock()
		numFiles := dc.dir.Len()
		version := dc.version
		dc.Unlock()

		return numFiles > 0, version, nil
	}, false)
}

type Fcond func() (retry bool, version uint64, err error)

func (dc *DirCache[E]) waitCond(cond Fcond, timed bool) error {
	dc.Init()
	for {
		retry, version, err := cond()
		if retry {
			if err := dc.waitChange(timed, version); err != nil {
				return err
			}
			continue
		}

		return err
	}
}

func (dc *DirCache[E]) waitChange(timed bool, startVersion uint64) error {
	const N = 2

	dc.Lock()
	defer dc.Unlock()

	nretry := 0
	for dc.version == startVersion && dc.err == nil && (!timed || nretry < N) {
		dc.hasEntries.Wait()
		nretry += 1
	}

	if dc.err != nil {
		db.DPrintf(dc.LSelector, "waitEntriesCond: error %v", dc.err)
		return dc.err
	}

	if timed && nretry >= N {
		db.DPrintf(db.TEST, "waitEntriesCond: timed out, stopped waiting %v", dc.LSelector)
		return serr.NewErr(serr.TErrNotfound, "no entries")
	}
	return nil
}

func (dc *DirCache[E]) checkErr() error {
	dc.Lock()
	defer dc.Unlock()

	if dc.err != nil {
		db.DPrintf(dc.LSelector, "checkErr %v", dc.err)
		return dc.err
	}
	return nil
}
