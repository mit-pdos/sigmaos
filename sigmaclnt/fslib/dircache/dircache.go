// Package dircache watches a changing directory and keeps a local copy
// with entries of type E chosen by the caller (e.g., rpcclnt's as in
// [rpcdirclnt]).  dircache updates the entries as files are
// created/removed in the watched directory.
package dircache

import (
	"sync"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sigmaclnt/fslib/dirreader"
	protsrv_proto "sigmaos/spproto/srv/proto"
	"sigmaos/util/sortedmapv1"
)

type NewValF[E any] func(string) (E, error)

type DirCache[E any] struct {
	*fslib.FsLib
	sync.RWMutex
	hasEntries    *sync.Cond
	dir           *sortedmapv1.SortedMap[string, E]
	isDone        atomic.Uint64
	isInit        atomic.Uint64
	Path          string
	LSelector     db.Tselector
	ESelector     db.Tselector
	newVal        NewValF[E]
	prefixFilters []string
	err           error
	ch            chan string
}

func NewDirCache[E any](fsl *fslib.FsLib, path string, newVal NewValF[E], ch chan string, lSelector db.Tselector, ESelector db.Tselector) *DirCache[E] {
	return NewDirCacheFilter(fsl, path, newVal, ch, lSelector, ESelector, nil)
}

// filter entries starting with prefix
func NewDirCacheFilter[E any](fsl *fslib.FsLib, path string, newVal NewValF[E], ch chan string, LSelector db.Tselector, ESelector db.Tselector, prefixes []string) *DirCache[E] {
	dc := &DirCache[E]{
		FsLib:         fsl,
		Path:          path,
		dir:           sortedmapv1.NewSortedMap[string, E](),
		LSelector:     LSelector,
		ESelector:     ESelector,
		newVal:        newVal,
		prefixFilters: prefixes,
		ch:            ch,
	}
	dc.hasEntries = sync.NewCond(&dc.RWMutex)
	return dc
}

func (dc *DirCache[E]) init() {
	if dc.isInit.Swap(1) == 0 && dc.isDone.Load() == 0 {
		dw := dc.initReadAndWatch()
		if dw == nil {
			return
		}
		go dc.watchDir(dw)
		go dc.watchdog()
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
	dc.init()
	if err := dc.checkErr(); err != nil {
		return 0, err
	}
	return dc.dir.Len(), nil
}

func (dc *DirCache[E]) GetEntries() ([]string, error) {
	dc.init()
	if err := dc.checkErr(); err != nil {
		return nil, err
	}
	return dc.dir.Keys(), nil
}

func (dc *DirCache[E]) WaitTimedEntriesN(n int) (int, error) {
	dc.init()
	if err := dc.checkErr(); err != nil {
		return 0, err
	}
	if err := dc.waitTimedEntriesN(n); err != nil {
		return 0, err
	}
	if dc.err != nil {
		return 0, dc.err
	}
	return dc.dir.Len(), nil
}

func (dc *DirCache[E]) WaitTimedGetEntriesN(n int) ([]string, error) {
	if _, err := dc.WaitTimedEntriesN(n); err != nil {
		return nil, err
	}
	return dc.dir.Keys(), nil
}

func (dc *DirCache[E]) GetEntry(n string) (E, error) {
	db.DPrintf(dc.LSelector, "GetEntry for %v", n)
	dc.init()

	if err := dc.checkErr(); err != nil {
		var e E
		db.DPrintf(dc.LSelector, "Done GetEntry for %v err %v", n, err)
		return e, err
	}
	var err error
	kok, e, vok := dc.dir.LookupKeyVal(n)
	if !kok {
		db.DPrintf(dc.LSelector, "Done GetEntry for %v kok %v", n, kok)
		serr.NewErr(serr.TErrNotfound, n)
	}
	if !vok {
		e, err = dc.allocVal(n)
	}
	db.DPrintf(dc.LSelector, "Done GetEntry for %v e %v err %v", n, e, err)
	return e, err
}

func (dc *DirCache[E]) LookupEntry(n string) error {
	_, err := dc.GetEntry(n)
	return err
}

func (dc *DirCache[E]) RandomEntry() (string, error) {
	var n string
	var ok bool

	db.DPrintf(dc.LSelector, "Random")
	dc.init()

	if err := dc.checkErr(); err != nil {
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

func (dc *DirCache[E]) WaitTimedRandomEntry() (string, error) {
	dc.init()
	return dc.waitEntry(dc.RandomEntry)
}

func (dc *DirCache[E]) RoundRobin() (string, error) {
	var n string
	var ok bool

	db.DPrintf(dc.LSelector, "RoundRobin")
	dc.init()

	if err := dc.checkErr(); err != nil {
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

func (dc *DirCache[E]) WaitTimedRoundRobin() (string, error) {
	dc.init()
	return dc.waitEntry(dc.RoundRobin)
}

func (dc *DirCache[E]) InvalidateEntry(name string) bool {
	dc.init()
	db.DPrintf(dc.LSelector, "InvalidateEntry %v", name)
	ok := dc.dir.Delete(name)
	db.DPrintf(dc.LSelector, "Done invalidate entry %v %v", ok, dc.dir)
	return ok
}

func (dc *DirCache[E]) allocVal(n string) (E, error) {
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

func (dc *DirCache[E]) waitEntry(selectF func() (string, error)) (string, error) {
	db.DPrintf(dc.LSelector, "waitEntry")
	for {
		n, err := selectF()
		if serr.IsErrorNotfound(err) {
			if sr := dc.waitTimedEntriesN(1); sr == nil {
				continue
			} else {
				err = sr
			}
		}
		db.DPrintf(dc.LSelector, "Done waitEntry %v %v", n, err)
		if err != nil {
			return "", err
		}
		return n, nil
	}
}

func (dc *DirCache[E]) waitTimedEntriesN(n int) error {
	const N = 2

	dc.Lock()
	defer dc.Unlock()

	nretry := 0
	l := dc.dir.Len()
	for dc.dir.Len() < n && dc.err == nil && nretry < N {
		dc.hasEntries.Wait()
		if dc.dir.Len() == l { // nothing changed; watchdog timeout
			nretry += 1
			continue
		}
		l = dc.dir.Len()
		nretry = 0
	}
	if nretry >= N {
		db.DPrintf(db.TEST, "waitTimedEntriesN: stop waiting %v", dc.LSelector)
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

func (dc *DirCache[E]) initReadAndWatch() *dirreader.DirWatcher {
	var initEnts []string
	var dw *dirreader.DirWatcher

	for dc.isDone.Load() == 0 {
		var err error
		initEnts, dw, err = dirreader.NewDirWatcherWithRead(dc.FsLib, dc.Path)
		if err != nil {
			if serr.IsErrorUnreachable(err) {
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
		dc.dir.InsertKey(ent)
		if dc.ch != nil {
			go func(ent string) {
				dc.ch <- ent
			}(ent)
		}
	}
	if dc.dir.Len() > 0 {
		dc.hasEntries.Broadcast()
	}
	dc.Unlock()

	return dw
}

// Monitor for changes to the directory and update the cached one
func (dc *DirCache[E]) watchDir(dw *dirreader.DirWatcher) {
	for event := range dw.Events() {
		dc.Lock()
		var madeChange bool
		switch event.Type {
			case protsrv_proto.WatchEventType_CREATE:
				madeChange = dc.dir.InsertKey(event.File)
				if madeChange && dc.ch != nil {
					go func(ent string) {
						dc.ch <- ent
					}(event.File)
				}
			case protsrv_proto.WatchEventType_REMOVE:
				madeChange = dc.dir.Delete(event.File)
				if madeChange {
					dc.hasEntries.Broadcast()
				}
		}
		dc.Unlock()
		if dc.isDone.Load() != 0 {
			break
		}
	}

	db.DPrintf(db.ALWAYS, "watchDir: done, closing %v", dw)
	dw.Close()
}
