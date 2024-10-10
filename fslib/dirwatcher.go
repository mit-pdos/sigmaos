package fslib

import (
	"bufio"
	"errors"
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
		return 0, errors.New("fd not found")
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
		currFiles := make([]string, 0)
		for _, st := range sts {
			currFiles = append(currFiles, st.Name)
		}

		prevFilesForFd, ok := prevFiles[fd]
		if !ok {
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

		db.DPrintf(db.WATCH, "read_: computed changes %s", sendString)

		prevFiles[fd] = currFiles		
		sendBytes := []byte(sendString)

		toSend[fd] = append(toSendForFd, sendBytes[:min(bufLen, len(sendBytes))]...)
		numCopied := copy(bytes, sendBytes)

		db.DPrintf(db.WATCH, "read_: wrote %s to buffer, %s is stored to send later", string(bytes), string(toSend[fd]))
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

func NewDirWatcher(fslib *FsLib, pn string) (*DirWatcher, error) {
	watchFd, err := fslib.dirWatch_(pn)
	if err != nil {
		return nil, err
	}

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
		return nil, err
	}
	for _, st := range sts {
		dw.changes[st.Name] = true
	}

	go func() {
		for scanner.Scan() {
			dw.Lock()
			defer dw.Unlock()

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

			dw.ents[name] = created
			dw.changes[name] = created

			dw.cond.Broadcast()
		}

		if err := scanner.Err(); err != nil {
			db.DPrintf(db.WATCH, "Reading watch stream produced err %v", err)
		}
	}()

	return dw, nil
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
	dw, err := NewDirWatcher(fsl, dir)
	if err != nil {
		return err
	}

	return dw.readDirWatch(watch)
}

func (dw *DirWatcher) readDirWatch(watch Fwatch_) error {
	dw.Lock()
	for watch(dw.ents, dw.changes) {
		// clear all changes
		clear(dw.changes)
		dw.cond.Wait()
	}
	dw.Unlock()

	err := dw.Close()
	if err != nil {
		return err
	}

	return nil
}

// Wait until pn isn't present
func (fsl *FsLib) WaitRemove_(pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH, "WaitRemove: readDirWatch dir %v\n", dir)
	err := fsl.readDirWatch_(dir, func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH, "WaitRemove %v %v %v\n", dir, ents, f)
		return ents[f]
	})
	return err
}

// Wait until pn exists
func (fsl *FsLib) WaitCreate_(pn string) error {
	dir := filepath.Dir(pn) + "/"
	f := filepath.Base(pn)
	db.DPrintf(db.WATCH, "WaitCreate: readDirWatch dir %v\n", dir)
	err := fsl.readDirWatch_(dir, func(ents map[string] bool, changes map[string]bool) bool {
		db.DPrintf(db.WATCH, "WaitCreate %v %v %v\n", dir, ents, f)
		return !ents[f]
	})
	return err
}

// Wait until n entries are in the directory
func (dw *DirWatcher) WaitNEntries(n int) error {
	err := dw.readDirWatch_(dw.pn, func(ents map[string]bool, changes map[string]bool) bool {
		return len(filterMap(ents)) < n
	})
	if err != nil {
		return err
	}
	return nil
}

// Watch for a directory change relative to present view and return
// all directory entries. Any file beginning with an excluded prefix
// are ignored. present should be sorted.
func (dw *DirWatcher) WatchEntriesChangedFilter(present []string, excludedPrefixes []string) ([]string, error) {
	var files = make([]string, 0)
	ix := 0
	err := dw.readDirWatch_(dw.pn, func(ents map[string]bool, changes map[string]bool) bool {
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


func (dw *DirWatcher) WatchEntriesChanged(present []string) ([]string, error) {
	return dw.WatchEntriesChangedFilter(present, nil)
}

// Watch for new entries, move them to the folder specified in dst, and return renamed entries.
func (dw *DirWatcher) WatchNewEntriesAndRename(dst string) ([]string, error) {
	var r error
	var newents []string
	 err := dw.readDirWatch_(dw.pn, func(ents map[string]bool, changes map[string]bool) bool {
		newents, r = dw.rename(filterMap(changes), dst)
		if r != nil || len(newents) > 0 {
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
	return newents, nil
}

// Takes each file and moves them to the dst directory. Returns a list of all
// files successfully moved
func (dw *DirWatcher) rename(files []string, dst string) ([]string, error) {
	var r error
	newents := make([]string, 0)
	for _, file := range files {
		if !dw.ents[file] {
			if err := dw.Rename(filepath.Join(dw.pn, file), filepath.Join(dst, file)); err == nil {
				newents = append(newents, file)
			} else if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
				r = err
				break
			}
		}
	}
	return newents, r
}
