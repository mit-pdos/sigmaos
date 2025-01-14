package dirreader

import (
	"bufio"
	"encoding/binary"
	"io"
	"maps"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	"sync"

	"sigmaos/serr"

	sp "sigmaos/sigmap"

	protsrv_proto "sigmaos/spproto/srv/proto"

	"google.golang.org/protobuf/proto"
)


type Fwatch func(ents map[string] bool, changes map[string] bool) bool

type DirReader struct {
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

func NewDirReader(fslib *fslib.FsLib, pn string) (*DirReader, error) {
	db.DPrintf(db.WATCH, "NewDirReader: Creating watch on %s", pn)

	fd, err := fslib.Open(pn, sp.OREAD)
	if err != nil {
		return nil, err
	}
	watchFd, err := fslib.DirWatch(fd)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.WATCH, "NewDirReader: Created watch on %s with fd=%d", pn, watchFd)

	reader := watchReader {
		fslib,
		watchFd,
	}
	bufferedReader := bufio.NewReader(reader)

	var mu sync.Mutex

	dr := &DirReader{
		FsLib:   fslib,
		Mutex:   &mu,
		pn:      pn,
		watchFd: watchFd,
		ents:    make(map[string]bool),
		changes: make(map[string]bool),
		closed:  false,
		reader:  bufferedReader,
	}

	sts, _, err := fslib.ReadDir(pn)
	if err != nil {
		return nil, err
	}
	for _, st := range sts {
		dr.ents[st.Name] = true
		dr.changes[st.Name] = true
	}

	if db.WillBePrinted(db.WATCH) {
		db.DPrintf(db.WATCH, "NewDirReader: Initial dir contents %v", dr.ents)
	}

	return dr, nil
}

// should hold lock for dr
func (dr *DirReader) isWatchClosed(err error) bool {
	return err != nil && (dr.closed || serr.IsErrCode(err, serr.TErrClosed) || serr.IsErrCode(err, serr.TErrUnreachable) || serr.IsErrCode(err, serr.TErrUnknownfid) || err == io.ErrUnexpectedEOF)
}

// should hold lock for dr
func (dr *DirReader) ReadUpdates() error {
	var length uint32
	err := binary.Read(dr.reader, binary.LittleEndian, &length)
	if dr.isWatchClosed(err) {
		db.DPrintf(db.WATCH, "DirReader ReadUpdates: watch stream for %s closed %v", dr.pn, err)
		return serr.NewErr(serr.TErrClosed, "")
	}
	if err != nil {
		db.DFatalf("failed to read length %v", err)
	}
	data := make([]byte, length)
	numRead, err := io.ReadFull(dr.reader, data)
	if dr.isWatchClosed(err) {
		db.DPrintf(db.WATCH, "DirReader ReadUpdates: watch stream for %s closed %v", dr.pn, err)
		return serr.NewErr(serr.TErrClosed, "")
	}
	if err != nil {
		db.DFatalf("watch stream produced err %v", err)
	}

	if uint32(numRead) != length {
		db.DFatalf("only received %d bytes, expected %d bytes", numRead, length)
	}

	eventList := &protsrv_proto.WatchEventList{}
	err = proto.Unmarshal(data, eventList)
	if err != nil {
		db.DFatalf("DirReader: failed to unmarshal data %v", err)
	}
	db.DPrintf(db.WATCH, "DirReader ReadUpdates: received %d bytes with %d events", numRead, len(eventList.Events))

	for _, event := range eventList.Events {
		switch event.Type {
		case protsrv_proto.WatchEventType_CREATE:
			dr.ents[event.File] = true
			dr.changes[event.File] = true
		case protsrv_proto.WatchEventType_REMOVE:
			dr.ents[event.File] = false
			dr.changes[event.File] = false
		default:
			db.DFatalf("DirReader: received unknown event type %v", event.Type)
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

// Gets the path of the directory being watched
func (dr *DirReader) GetPath() string {
	return dr.pn
}

// Gets the list of files in the directory as of the latest watch. Can be stale
// if changes have been made since the last watch operation
func (dr *DirReader) GetDir() ([]string, error) {
	dr.Lock()
	defer dr.Unlock()

	return filterMap(dr.ents), nil
}

func (dr *DirReader) Close() error {
	db.DPrintf(db.WATCH, "DirReader: closing watch on %s", dr.pn)
	dr.closed = true
	return dr.CloseFd(dr.watchFd)
}

// Keep reading dir until wait returns false (e.g., a new file has
// been created in dir).
func (dr *DirReader) readDirWatch(watch Fwatch) error {
	dr.Lock()
	defer dr.Unlock()

	for watch(dr.ents, dr.changes) {
		dr.changes = make(map[string]bool)
		err := dr.ReadUpdates()
		if err != nil {
			db.DPrintf(db.WATCH, "DirReader readDirWatch: ReadUpdates failed %v", err)
			return err
		}
	}
	dr.changes = make(map[string]bool)

	return nil
}

// Blocks until file exists in the directory
func (dr *DirReader) WaitCreate(file string) error {
	db.DPrintf(db.WATCH, "DirReader WaitCreate: dir %s file %s", dr.pn, file)

	err := dr.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		return !ents[file]
	})
	return err
}

// Blocks until file is no longer in the directory
func (dr *DirReader) WaitRemove(file string) error {
	db.DPrintf(db.WATCH, "DirReader WaitRemove: dir %s file %s", dr.pn, file)
	firstTime := true
	err := dr.readDirWatch(func(ents map[string] bool, changes map[string]bool) bool {
		if firstTime {
			firstTime = false
			return ents[file]
		} else {
			created, ok := changes[file]
			if !ok {
				return true
			}
			return created
		}
	})
	return err
}

// Blocks until at least n entries to be in the directory
func (dr *DirReader) WaitNEntries(n int) error {
	db.DPrintf(db.WATCH, "DirReader WaitNEntries: dir %s n %d", dr.pn, n)
	err := dr.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		return len(filterMap(ents)) < n
	})
	if err != nil {
		return err
	}
	return nil
}

// Blocks until the directory to be empty
func (dr *DirReader) WaitEmpty() error {
	db.DPrintf(db.WATCH, "DirReader WaitEmpty: dir %s", dr.pn)
	err := dr.readDirWatch(func(ents map[string]bool, changes map[string]bool) bool {
		return len(filterMap(ents)) > 0
	})
	if err != nil {
		return err
	}
	return nil
}

// Watch for a directory change and then return all directory entry changes since the last call to
// a Watch method.
func (dr *DirReader) WatchEntriesChanged() (map[string]bool, error) {
	var ret map[string]bool
	db.DPrintf(db.WATCH, "DirReader WatchEntriesChanged: dir %v\n", dr.pn)
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

// Uses rename to move all entries in the directory to dst. If there are no entries to be renamed,
// blocks until a new entry is added and then moves it.
func (dr *DirReader) WatchNewEntriesAndRename(dst string) ([]string, error) {
	var r error
	presentFiles := filterMap(dr.ents)
	if db.WillBePrinted(db.WATCH) {
		db.DPrintf(db.WATCH, "DirReader WatchNewEntriesAndRename: dir %v, present: %v, dst %v\n", dr.pn, presentFiles, dst)
	}
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

// Uses rename to move all entries in the directory to dst. Can be potentially stale, so combining it
// with other Watch calls may be desired to get the cache up to date.
// Does not block if there are no entries to rename
func (dr *DirReader) GetEntriesAndRename(dst string) ([]string, error) {
	presentFiles := filterMap(dr.ents)
	if db.WillBePrinted(db.WATCH) {
		db.DPrintf(db.WATCH, "DirReader GetEntriesAndRename: dir %v, present: %v, dst %v\n", dr.pn, presentFiles, dst)
	}
	return dr.rename(presentFiles, dst)
}

// Takes each file and moves them to the dst directory. Returns a list of all
// files successfully moved
func (dr *DirReader) rename(files []string, dst string) ([]string, error) {
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
