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

	dr := &DirReaderV2{
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
		dr.ents[st.Name] = true
		dr.changes[st.Name] = true
	}

	if db.WillBePrinted(db.WATCH_V2) {
		db.DPrintf(db.WATCH_V2, "NewDirReaderV2: Initial dir contents %v", dr.ents)
	}

	return dr, nil
}

// should hold lock for dr
func (dr *DirReaderV2) isWatchClosed(err error) bool {
	return err != nil && (dr.closed || serr.IsErrCode(err, serr.TErrClosed) || serr.IsErrCode(err, serr.TErrUnreachable) || serr.IsErrCode(err, serr.TErrUnknownfid) || err == io.ErrUnexpectedEOF)
}

// should hold lock for dr
func (dr *DirReaderV2) ReadUpdates() error {
	var length uint32
	err := binary.Read(dr.reader, binary.LittleEndian, &length)
	if dr.isWatchClosed(err) {
		db.DPrintf(db.WATCH_V2, "DirReaderV2: Watch stream for %s closed %v", dr.pn, err)
		return serr.NewErr(serr.TErrClosed, "")
	}
	if err != nil {
		db.DFatalf("DirReaderV2: failed to read length %v", err)
	}
	data := make([]byte, length)
	numRead, err := io.ReadFull(dr.reader, data)
	if dr.isWatchClosed(err) {
		db.DPrintf(db.WATCH_V2, "DirReaderV2: Watch stream for %s closed %v", dr.pn, err)
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
	db.DPrintf(db.WATCH_V2, "DirReaderV2: received %d bytes with %d events", numRead, len(eventList.Events))

	for _, event := range eventList.Events {
		switch event.Type {
		case protsrv_proto.WatchEventType_CREATE:
			dr.ents[event.File] = true
			dr.changes[event.File] = true
		case protsrv_proto.WatchEventType_REMOVE:
			dr.ents[event.File] = false
			dr.changes[event.File] = false
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

func (dr *DirReaderV2) GetPath() string {
	return dr.pn
}

func (dr *DirReaderV2) GetDir() ([]string, error) {
	dr.Lock()
	defer dr.Unlock()

	return filterMap(dr.ents), nil
}

func (dr *DirReaderV2) Close() error {
	db.DPrintf(db.WATCH_V2, "Closing watch on %s", dr.pn)
	dr.closed = true
	return dr.CloseFd(dr.watchFd)
}

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir).
func (dr *DirReaderV2) readDirWatch(watch FwatchV2) error {
	dr.Lock()
	defer dr.Unlock()

	for watch(dr.ents, dr.changes) {
		dr.changes = make(map[string]bool)
		err := dr.ReadUpdates()
		if err != nil {
			db.DPrintf(db.WATCH_V2, "readDirWatch: ReadUpdates failed %v", err)
			return err
		}
	}
	dr.changes = make(map[string]bool)

	return nil
}

func (dr *DirReaderV2) WaitRemove(file string) error {
	db.DPrintf(db.WATCH_V2, "WaitRemove: dir %s file %s", dr.pn, file)
	err := dr.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		return ents[file]
	})
	return err
}

func (dr *DirReaderV2) WaitCreate(file string) error {
	db.DPrintf(db.WATCH_V2, "WaitCreate: dir %s file %s", dr.pn, file)
	err := dr.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		return !ents[file]
	})
	return err
}

func (dr *DirReaderV2) WaitNEntries(n int) error {
	db.DPrintf(db.WATCH_V2, "WaitNEntries: dir %s n %d", dr.pn, n)
	err := dr.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		return len(filterMap(ents)) < n
	})
	if err != nil {
		return err
	}
	return nil
}

func (dr *DirReaderV2) WaitEmpty() error {
	db.DPrintf(db.WATCH_V2, "WaitEmpty: dir %s", dr.pn)
	err := dr.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		return len(filterMap(ents)) > 0
	})
	if err != nil {
		return err
	}
	return nil
}

func (dr *DirReaderV2) WatchEntriesChangedRelative(present []string, excludedPrefixes []string) ([]string, bool, error) {
	var files = make([]string, 0)
	db.DPrintf(db.WATCH_V2, "WatchUniqueEntries: dir %v, present: %v, excludedPrefixes %v\n", dr.pn, present, excludedPrefixes)
	var ret []string
	err := dr.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
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

func (dr *DirReaderV2) WatchEntriesChanged() (map[string]bool, error) {
	var ret map[string]bool
	err := dr.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
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

func (dr *DirReaderV2) WatchNewEntriesAndRename(dst string) ([]string, error) {
	var r error
	presentFiles := filterMap(dr.ents)
	db.DPrintf(db.WATCH_V2, "WatchNewEntriesAndRename: dir %v, present: %v, dst %v\n", dr.pn, presentFiles, dst)
	if len(presentFiles) > 0 {
		return dr.rename(presentFiles, dst)
	}

	var movedEnts []string
	err := dr.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		movedEnts, r = dr.rename(filterMap(changes), dst)
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


func (dr *DirReaderV2) GetEntriesAndRename(dst string) ([]string, error) {
	presentFiles := filterMap(dr.ents)
	return dr.rename(presentFiles, dst)
}

// Takes each file and moves them to the dst directory. Returns a list of all
// files successfully moved
func (dr *DirReaderV2) rename(files []string, dst string) ([]string, error) {
	var r error
	newents := make([]string, 0)
	for _, file := range files {
		if dr.ents[file] {
			if err := dr.Rename(filepath.Join(dr.pn, file), filepath.Join(dst, file)); err == nil {
				newents = append(newents, file)
			} else if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
				r = err
				break
			}

			// either we successfully renamed it or another proc renamed it first
			dr.ents[file] = false
		}
	}
	return newents, r
}
