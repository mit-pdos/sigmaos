package dirreader

import (
	"bufio"
	"encoding/binary"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/serr"
	sp "sigmaos/sigmap"

	protsrv_proto "sigmaos/protsrv/proto"

	"google.golang.org/protobuf/proto"
)

// TODO: figure out which tests if any are broken by this branch
// TODO: write more small tests for each of the functions

type FwatchV2 func(ents map[string] bool, changes map[string] bool) bool

type DirReaderV2 struct {
	*fslib.FsLib
	*sync.Mutex
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

func isWatchClosed(err error) bool {
	return err != nil && (serr.IsErrCode(err, serr.TErrClosed) || serr.IsErrCode(err, serr.TErrUnreachable) || serr.IsErrCode(err, serr.TErrUnknownfid) || err == io.ErrUnexpectedEOF)
}

// should hold lock for dw
func (dw *DirReaderV2) ReadUpdates() error {
	var length uint32
	err := binary.Read(dw.reader, binary.LittleEndian, &length)
	if isWatchClosed(err) {
		db.DPrintf(db.WATCH_V2, "DirReaderV2: Watch stream for %s closed %v", dw.pn, err)
		return serr.NewErr(serr.TErrClosed, "")
	}
	if err != nil {
		db.DFatalf("DirReaderV2: failed to read length %v", err)
	}
	data := make([]byte, length)
	numRead, err := io.ReadFull(dw.reader, data)
	if isWatchClosed(err) {
		db.DPrintf(db.WATCH_V2, "DirReaderV2: Watch stream for %s closed %v", dw.pn, err)
		return serr.NewErr(serr.TErrClosed, "")
	}
	if err != nil {
		db.DFatalf("DirReaderV2: Watch stream produced err %v", err)
	}

	if uint32(numRead) != length {
		db.DFatalf("DirReaderV2: only received %d bytes, expected %d bytes", numRead, length)
	}

	eventList := &protsrv_proto.WatchEventList{}
	err = proto.Unmarshal(data, eventList)
	if err != nil {
		db.DFatalf("DirReaderV2: failed to unmarshal data %v", err)
	}

	for _, event := range eventList.Events {
		switch event.Type {
		case protsrv_proto.WatchEventType_CREATE:
			dw.ents[event.File] = true
			dw.changes[event.File] = true
		case protsrv_proto.WatchEventType_REMOVE:
			dw.ents[event.File] = false
			dw.changes[event.File] = false
		default:
			db.DFatalf("DirReaderV2: received unknown event type %v", event.Type)
		}
	}

	return nil
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

func (dw *DirReaderV2) WaitRemove(file string) error {
	err := dw.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		return ents[file]
	})
	return err
}

func (dw *DirReaderV2) WaitCreate(file string) error {
	err := dw.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		return !ents[file]
	})
	return err
}

func (dw *DirReaderV2) WaitNEntries(n int) error {
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		return len(filterMap(ents)) < n
	})
	if err != nil {
		return err
	}
	return nil
}

func (dw *DirReaderV2) WaitEmpty() error {
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		return len(filterMap(ents)) > 0
	})
	if err != nil {
		return err
	}
	return nil
}

func (dw *DirReaderV2) WatchEntriesChangedRelative(present []string, excludedPrefixes []string) ([]string, bool, error) {
	var files = make([]string, 0)
	db.DPrintf(db.WATCH, "WatchUniqueEntries: dir %v, present: %v, excludedPrefixes %v\n", dw.pn, present, excludedPrefixes)
	var ret []string
	err := dw.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		unchanged := true
		files = filterMap(changes)
		slices.Sort(files)
		ret = make([]string, 0)
		ix := 0
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
			ret = append(ret, file)

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
		return nil, true, err
	}
	return ret, true, nil
}

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

func (dw *DirReaderV2) WatchNewEntriesAndRename(dst string) ([]string, error) {
	var r error
	presentFiles := filterMap(dw.ents)
	db.DPrintf(db.WATCH, "WatchNewEntriesAndRename: dir %v, present: %v, dst %v\n", dw.pn, presentFiles, dst)
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
