package dirreader

import (
	"bufio"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type FwatchV2 func(ents map[string] bool, changes map[string] bool) bool

type DirReaderV2 struct {
	*fslib.FsLib
	*sync.Mutex
	cond *sync.Cond
	pn string
	watchFd int
	ents map[string]bool
	changes map[string]bool
	closed bool
	reader *bufio.Reader
}

type watchReader struct {
	*fslib.FsLib
	watchFd int
}

func (wr watchReader) Read(p []byte) (int, error) {
	size, err := wr.FsLib.Read(wr.watchFd, p)
	return int(size), err
}

// TODO: update it so that this and the original dirreader have a unified interface and then swap between them with a flag


var CREATE_PREFIX = "CREATE "
var REMOVE_PREFIX = "REMOVE "

func NewDirReaderV2(fslib *fslib.FsLib, pn string) (*DirReaderV2, error) {
	db.DPrintf(db.WATCH_V2, "Creating new watch on %s", pn)

	fd, err := fslib.Open(pn, sp.OREAD)
	if err != nil {
		return nil, err
	}
	watchFd, err := fslib.DirWatchV2(fd)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.WATCH_V2, "Created watch on %s with fd=%d", pn, watchFd)

	reader := watchReader {
		fslib,
		watchFd,
	}
	bufferedReader := bufio.NewReader(reader)

	var mu sync.Mutex

	dw := &DirReaderV2{
		FsLib: fslib,
		Mutex: &mu,
		pn:    pn,
		watchFd: watchFd,
		ents:   make(map[string]bool),
		changes: make(map[string]bool),
		closed: false,
		reader: bufferedReader,
	}

	sts, _, err := fslib.ReadDir(pn)
	if err != nil {
		return nil, err
	}
	for _, st := range sts {
		dw.ents[st.Name] = true
		dw.changes[st.Name] = true
	}

	db.DPrintf(db.WATCH_V2, "NewDirReaderV2: Initial dir contents %v", dw.ents)

	return dw, nil
}

// should hold lock for dw
func (dw *DirReaderV2) ReadUpdates() error {
	err := dw.ReadNextUpdate()
	if err != nil {
		return err
	}

	// read extra events as long as they do not cause is to incur an additional RPC read()
	hasUpdates, err := dw.HasUpdateAvailableInBuffer()
	for hasUpdates && err == nil {
		err = dw.ReadNextUpdate()
		if err != nil {
			return err
		}

		hasUpdates, err = dw.HasUpdateAvailableInBuffer()
	}

	return err
}

// should hold lock for dw
func (dw *DirReaderV2)  ReadNextUpdate() error {
	event, err := dw.reader.ReadString('\n')

	if dw.closed {
		return serr.NewErr(serr.TErrClosed, "")
	}

	if err != nil {
		if serr.IsErrCode(err, serr.TErrClosed) || serr.IsErrCode(err, serr.TErrUnreachable) || serr.IsErrCode(err, serr.TErrUnknownfid) {
			db.DPrintf(db.WATCH_V2, "DirWatcher: Watch stream for %s closed", dw.pn)
			return serr.NewErr(serr.TErrClosed, "")
		} else {
			db.DFatalf("DirWatcher: Watch stream produced err %v", err)
		}
	}

	// remove the newline
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

	if dw.closed {
		return serr.NewErr(serr.TErrClosed, "")
	}

	db.DPrintf(db.WATCH_V2, "DirWatcher: Read event %s %t", name, created)

	dw.ents[name] = created
	dw.changes[name] = created

	return nil
}


// should hold lock for dw
func (dw *DirReaderV2) HasUpdateAvailableInBuffer() (bool, error) {
	buffer, err := dw.reader.Peek(dw.reader.Buffered())
	if err != nil {
		db.DPrintf(db.WATCH_V2, "DirWatcher: failed to peek at buffer for %s", dw.pn)
		return false, err
	}

	for _, b := range buffer {
		if b == '\n' {
			return true, nil
		}
	}

	return false, nil
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

func (dw *DirReaderV2) GetPath() string {
	return dw.pn
}

func (dw *DirReaderV2) GetDir() ([]string, error) {
	dw.Lock()
	defer dw.Unlock()

	return filterMap(dw.ents), nil
}

func (dw *DirReaderV2) Close() error {
	db.DPrintf(db.WATCH_V2, "Closing watch on %s", dw.pn)
	dw.closed = true
	return dw.CloseFd(dw.watchFd)
}

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir).
func (dw *DirReaderV2) readDirWatch(watch FwatchV2) error {
	dw.Lock()
	defer dw.Unlock()

	db.DPrintf(db.WATCH_V2, "readDirWatch: initial dir contents %v", dw.ents)

	for watch(dw.ents, dw.changes) {
		clear(dw.changes)
		err := dw.ReadUpdates()
		if err != nil {
			db.DPrintf(db.WATCH_V2, "readDirWatch: ReadUpdates failed %v", err)
			return err
		}
	}
	clear(dw.changes)

	return nil
}

// Wait until pn isn't present
func (dw *DirReaderV2) WaitRemove(file string) error {
	err := dw.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_V2, "WaitRemove %v %v\n", ents, file)
		return ents[file]
	})
	return err
}

// Wait until pn exists
func (dw *DirReaderV2) WaitCreate(file string) error {
	err := dw.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_V2, "WaitCreate %v %v %t\n", ents, file, ents[file])
		return !ents[file]
	})
	return err
}

// Wait until n entries are in the directory
func (dw *DirReaderV2) WaitNEntries(n int) error {
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_V2, "WaitNEntries: %v %v", ents, changes)
		return len(filterMap(ents)) < n
	})
	if err != nil {
		return err
	}
	return nil
}

// Wait until directory is empty
func (dw *DirReaderV2) WaitEmpty() error {
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_V2, "WaitEmpty: %v %v", ents, changes)
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
func (dw *DirReaderV2) WatchEntriesChangedRelative(present []string, excludedPrefixes []string) ([]string, bool, error) {
	var files = make([]string, 0)
	ix := 0

	db.DPrintf(db.WATCH, "WatchUniqueEntries: dir %v, present: %v, excludedPrefixes %v\n", dw.pn, present, excludedPrefixes)
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		unchanged := true
		files = filterMap(ents)
		slices.Sort(files)
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
		db.DPrintf(db.WATCH, "WatchUniqueEntries: ents: %v, present: %v, files: %v, unchanged: %v\n", ents, present, files, unchanged)
		return unchanged
	})
	if err != nil {
		return nil, true, err
	}
	return files, true, nil
}

// Watch for a directory change and then only return new changes since the last call to a Watch
func (dw *DirReaderV2) WatchEntriesChanged() (map[string]bool, error) {
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
func (dw *DirReaderV2) WatchNewEntriesAndRename(dst string) ([]string, error) {
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


// Uses rename to move all entries in the directory to dst. Does not block if there are no entries to rename
func (dw *DirReaderV2) GetEntriesAndRename(dst string) ([]string, error) {
	presentFiles := filterMap(dw.ents)
	return dw.rename(presentFiles, dst)
}

// Takes each file and moves them to the dst directory. Returns a list of all
// files successfully moved
func (dw *DirReaderV2) rename(files []string, dst string) ([]string, error) {
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
