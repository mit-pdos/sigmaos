package fslib

import (
	"bufio"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

var watchLookup map[int]string = make(map[int]string)
var maxId int

var prevFiles map[int][]string = make(map[int][]string)
var toSend map[int][]byte = make(map[int][]byte)

func (fsl *FsLib) read_(fd int, bytes []byte) (sp.Tsize, error) {
	dir, ok := watchLookup[fd]
	if !ok {
		return 0, fmt.Errorf("fd %d not found", fd)
	}

	bufLen := len(bytes)

	toSendForFd, ok := toSend[fd]
	if ok && len(toSendForFd) > 0 {
		toSend[fd] = toSendForFd[min(bufLen, len(toSendForFd)):]
		numCopied := copy(bytes, toSendForFd)
		return sp.Tsize(numCopied), nil
	}

	for {
		sts, rdr, err := fsl.ReadDir(dir)
		if err != nil {
			return 0, err
		}
		currFilesSet := make(map[string]bool)
		for _, st := range sts {
			currFilesSet[st.Name] = true
		}
		currFiles := make([]string, 0)
		for file := range currFilesSet {
			currFiles = append(currFiles, file)
		}

		// TODO: should this be how it works? if there are files should I send creates for all of them?
		prevFilesForFd, ok := prevFiles[fd]
		if !ok {
			db.DPrintf(db.WATCH_NEW, "read_: No prev view of directory found, no changes computed")
			prevFiles[fd] = currFiles
			continue
		}

		addedFiles := make([]string, 0)
		deletedFiles := make([]string, 0)

		for _, prevFile := range prevFilesForFd {
			found := false
			for _, currFile := range currFiles {
				if prevFile == currFile {
					found = true
				}
			}

			if !found {
				deletedFiles = append(deletedFiles, prevFile)
			}
		}

		for _, currFile := range currFiles {
			found := false
			for _, prevFile := range prevFilesForFd {
				if prevFile == currFile {
					found = true
				}
			}

			if !found {
				addedFiles = append(addedFiles, currFile)
			}
		}

		// if no changes, wait for changes and try again
		if len(addedFiles) + len(deletedFiles) == 0 {
			if err := fsl.DirWatch(rdr.fd); err != nil {
				// TODO what does this mean?
				if serr.IsErrCode(err, serr.TErrVersion) {
					db.DPrintf(db.WATCH_NEW, "read_: Version mismatch %v", dir)
					continue
				}
				return 0, err
			}
			continue
		}

		sendString := ""
		for _, addedFile := range addedFiles {
			sendString += CREATE_PREFIX + addedFile + "\n"
		}
		for _, deletedFiles := range deletedFiles {
			sendString += REMOVE_PREFIX + deletedFiles + "\n"
		}

		db.DPrintf(db.WATCH_NEW, "read_: computed changes %s", sendString)

		prevFiles[fd] = currFiles		
		sendBytes := []byte(sendString)

		// limited by size of buffer
		numCopied := copy(bytes, sendBytes)

		// store everything else to be sent upon the next read
		toSend[fd] = append(toSendForFd, sendBytes[min(bufLen, len(sendBytes)):]...)

		db.DPrintf(db.WATCH_NEW, "read_: wrote %s to buffer (%d bytes), %s is stored to send later", string(bytes), numCopied, string(toSend[fd]))
		return sp.Tsize(numCopied), nil
	}
}

func (fsl *FsLib) close_(fd int) error {
	delete(watchLookup, fd)
	return nil
}

func (fsl *FsLib) dirWatch_(dir string) (int, error) {
	id := maxId
	watchLookup[id] = dir
	maxId += 1
	return id, nil
}

type Fwatch_ func(ents map[string] bool, changes map[string] bool) bool

type DirWatcher struct {
	*FsLib
	*sync.Mutex
	cond *sync.Cond
	pn string
	watchFd int
	ents map[string]bool
	changes map[string]bool
}

type watchReader struct {
	*FsLib
	watchFd int
}

func (wr watchReader) Read(p []byte) (int, error) {
	size, err := wr.read_(wr.watchFd, p)
	return int(size), err
}

var CREATE_PREFIX = "CREATE "
var REMOVE_PREFIX = "REMOVE "

func NewDirWatcher(fslib *FsLib, pn string) (*DirWatcher, []string, error) {
	db.DPrintf(db.WATCH_NEW, "Creating new watch on %s", pn)

	watchFd, err := fslib.dirWatch_(pn)
	if err != nil {
		return nil, nil, err
	}

	db.DPrintf(db.WATCH_NEW, "Created watch on %s with fd=%d", pn, watchFd)

	reader := watchReader {
		fslib,
		watchFd,
	}
	scanner := bufio.NewScanner(reader)

	var mu sync.Mutex

	dw := &DirWatcher{
		FsLib: fslib,
		Mutex: &mu,
		cond:  sync.NewCond(&mu),
		pn:    pn,
		watchFd: watchFd,
		ents:   make(map[string]bool),
		changes: make(map[string]bool),
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
			for scanner.Scan() {
				event := scanner.Text()

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

				db.DPrintf(db.WATCH_NEW, "DirWatcher: Broadcasting event %s %t", name, created)

				dw.ents[name] = created
				dw.changes[name] = created

				dw.cond.Broadcast()

				dw.Unlock()
			}

			if err := scanner.Err(); err != nil {
				db.DPrintf(db.WATCH_NEW, "DirWatcher: Reading watch stream produced err: %v", err)
				return
			}
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
	return dw.close_(dw.watchFd)
}

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir).
func (fsl *FsLib) readDirWatch_(dir string, watch Fwatch_) error {
	dw, _, err := NewDirWatcher(fsl, dir)
	if err != nil {
		return err
	}

	err = dw.readDirWatch_(watch)
	if err != nil {
		db.DPrintf(db.WATCH_NEW, "readDirWatch for %s produced error %v", dir, err)
	}

	err = dw.Close()
	if err != nil {
		db.DPrintf(db.WATCH_NEW, "Failed to close watch for %s: %v", dir, err)
	}
	return err
}

func (dw *DirWatcher) readDirWatch_(watch Fwatch_) error {
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
func (fsl *FsLib) WaitRemove_(pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH_NEW, "WaitRemove: readDirWatch dir %v\n", dir)
	err := fsl.readDirWatch_(dir, func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_NEW, "WaitRemove %v %v %v\n", dir, ents, f)
		return ents[f]
	})
	return err
}

// Wait until pn exists
func (fsl *FsLib) WaitCreate_(pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH_NEW, "WaitCreate: readDirWatch dir %v\n", dir)
	err := fsl.readDirWatch_(dir, func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_NEW, "WaitCreate %v %v %v\n", dir, ents, f)
		return !ents[f]
	})
	return err
}

// Wait until n entries are in the directory
func (dw *DirWatcher) WaitNEntries(n int) error {
	err := dw.readDirWatch_(func(ents map[string]bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH_NEW, "WaitNEntries: %v %v", ents, changes)
		return len(filterMap(ents)) < n
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
	err := dw.readDirWatch_(func(ents map[string]bool, changes map[string]bool) bool {
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
	err := dw.readDirWatch_(func(ents map[string]bool, changes map[string]bool) bool {
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
	err := dw.readDirWatch_(func(ents map[string]bool, changes map[string]bool) bool {
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
