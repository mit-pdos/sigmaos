package fslib

import (
	"bufio"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Fwatch_ func(ents map[string] bool, changes map[string] bool) bool

type DirWatcher struct {
	*FsLib
	*sync.Mutex
	cond *sync.Cond
	pn string
	watchFd int
	ents map[string]bool
	changes map[string]bool
	closed bool
}

type watchReader struct {
	*FsLib
	watchFd int
}

func (wr watchReader) Read(p []byte) (int, error) {
	size, err := wr.FsLib.Read(wr.watchFd, p)
	return int(size), err
}

var CREATE_PREFIX = "CREATE "
var REMOVE_PREFIX = "REMOVE "

func NewDirWatcher(fslib *FsLib, pn string) (*DirWatcher, []string, error) {
	db.DPrintf(db.WATCH_NEW, "Creating new watch on %s", pn)

	fd, err := fslib.Open(pn, sp.OREAD)
	if err != nil {
		return nil, nil, err
	}
	watchFd, err := fslib.DirWatchV2(fd)
	if err != nil {
		return nil, nil, err
	}

	db.DPrintf(db.WATCH_NEW, "Created watch on %s with fd=%d", pn, watchFd)

	reader := watchReader {
		fslib,
		watchFd,
	}
	bufferedReader := bufio.NewReader(reader)

	var mu sync.Mutex

	dw := &DirWatcher{
		FsLib: fslib,
		Mutex: &mu,
		cond:  sync.NewCond(&mu),
		pn:    pn,
		watchFd: watchFd,
		ents:   make(map[string]bool),
		changes: make(map[string]bool),
		closed: false,
	}

	sts, _, err := fslib.ReadDir(pn)
	if err != nil {
		return nil, nil, err
	}
	for _, st := range sts {
		dw.ents[st.Name] = true
		dw.changes[st.Name] = true
	}

	files := filterMap(dw.ents)

	go func() {
		for {
			event, err := bufferedReader.ReadString('\n')
			if dw.closed {
				return
			}

			// remove the newline
			if err != nil {
				if serr.IsErrCode(err, serr.TErrClosed) {
					db.DPrintf(db.WATCH_NEW, "DirWatcher: Watch stream for %s closed", pn)
					return
				} else {
					db.DFatalf("DirWatcher: Watch stream produced err %v", err)
				}
			}

			event = event[:len(event) - 1]
			var name string
			var created bool

			if strings.HasPrefix(event, CREATE_PREFIX) {
				name = event[len(CREATE_PREFIX):]
				created = true
			} else if strings.HasPrefix(event, REMOVE_PREFIX) {
				name = event[len(REMOVE_PREFIX):]
				created = false
			} else {
				db.DFatalf("Received malformed watch event: %s", event)
			}

			dw.Lock()
			if dw.closed {
				dw.Unlock()
				return
			}

			db.DPrintf(db.WATCH_NEW, "DirWatcher: Broadcasting event %s %t", name, created)

			dw.ents[name] = created
			dw.changes[name] = created

			dw.cond.Broadcast()

			dw.Unlock()
		}
	}()

	return dw, files, nil
}

func filterMap(ents map[string] bool) []string {
	var result []string

	for filename, exists := range ents {
		if exists {
			result = append(result, filename)
		}
	}
	return result
}

func (dw *DirWatcher) GetDir() []string {
	dw.Lock()
	defer dw.Unlock()

	return filterMap(dw.ents)
}

func (dw *DirWatcher) Close() error {
	dw.closed = true
	return dw.CloseFd(dw.watchFd)
}

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir).
func (dw *DirWatcher) readDirWatch(watch Fwatch_) error {
	dw.Lock()
	for watch(dw.ents, dw.changes) {
		clear(dw.changes)
		dw.cond.Wait()
	}
	clear(dw.changes)
	dw.Unlock()

	return nil
}

// Wait until pn isn't present
func (dw *DirWatcher) WaitRemove(file string) error {
	err := dw.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_NEW, "WaitRemove %v %v\n", ents, file)
		return ents[file]
	})
	return err
}

// Wait until pn exists
func (dw *DirWatcher) WaitCreate(file string) error {
	err := dw.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_NEW, "WaitCreate %v %v %t\n", ents, file, ents[file])
		return !ents[file]
	})
	return err
}

// Wait until n entries are in the directory
func (dw *DirWatcher) WaitNEntries(n int) error {
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_NEW, "WaitNEntries: %v %v", ents, changes)
		return len(filterMap(ents)) < n
	})
	if err != nil {
		return err
	}
	return nil
}

// Wait until direcotry is empty
func (dw *DirWatcher) WaitEmpty() error {
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_NEW, "WaitEmpty: %v %v", ents, changes)
		return len(filterMap(ents)) > 0
	})
	if err != nil {
		return err
	}
	return nil
}

// Watch for a directory change relative to present view and then return
// all directory entries. If provided, any file beginning with an
// excluded prefix is ignored. present should be sorted.
func (dw *DirWatcher) WatchEntriesChangedRelativeFiltered(present []string, excludedPrefixes []string) ([]string, error) {
	var files = make([]string, 0)
	ix := 0
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		unchanged := true
		filesPresent := filterMap(ents)
		slices.Sort(filesPresent)
		for _, file := range files {
			skip := false
			for _, pf := range excludedPrefixes {
				if strings.HasPrefix(file, pf) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			for ix < len(present) && present[ix] < file {
				ix += 1
			}
			if ix >= len(present) || present[ix] != file {
				unchanged = false
			}
		}
		return unchanged
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Watch for a directory change relative to present view and then return 
// all directory entries. present should be sorted
func (dw *DirWatcher) WatchEntriesChangedRelative(present []string) ([]string, error) {
	return dw.WatchEntriesChangedRelativeFiltered(present, nil)
}

// Watch for a directory change and then only return new changes since the last call to a Watch
func (dw *DirWatcher) WatchEntriesChanged() (map[string]bool, error) {
	var ret map[string]bool
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		if len(changes) > 0 {
			ret = maps.Clone(changes)
			return false
		} else {
			return true
		}
	})

	if err != nil {
		return nil, err
	} else {
		return ret, nil
	}
}

// Uses rename to move all entries in the directory to dst. If there are no further entries to be renamed,
// waits for a new entry and moves it.
func (dw *DirWatcher) WatchEntriesAndRename(dst string) ([]string, error) {
	var r error
	presentFiles := filterMap(dw.ents)
	if len(presentFiles) > 0 {
		return dw.rename(presentFiles, dst)
	}

	var movedEnts []string
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		movedEnts, r = dw.rename(filterMap(changes), dst)
		if r != nil || len(movedEnts) > 0 {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	if r != nil {
		return nil, r
	}
	return movedEnts, nil
}

// Takes each file and moves them to the dst directory. Returns a list of all
// files successfully moved
func (dw *DirWatcher) rename(files []string, dst string) ([]string, error) {
	var r error
	newents := make([]string, 0)
	for _, file := range files {
		if dw.ents[file] {
			if err := dw.Rename(filepath.Join(dw.pn, file), filepath.Join(dst, file)); err == nil {
				newents = append(newents, file)
			} else if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
				r = err
				break
			}

			// either we successfully renamed it or another proc renamed it first
			dw.ents[file] = false
		}
	}
	return newents, r
}
