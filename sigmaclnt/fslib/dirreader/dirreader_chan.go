package dirreader

import (
	"bufio"
	"encoding/binary"
	"io"
	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sync/atomic"

	protsrv_proto "sigmaos/spproto/srv/proto"

	"google.golang.org/protobuf/proto"
)

type DirWatcher struct {
	*fslib.FsLib
	ch chan *protsrv_proto.WatchEvent
	pn string
	closed atomic.Bool
	reader *bufio.Reader
	watchFd int
}

func NewDirWatcher(fslib *fslib.FsLib, pn string) (*DirWatcher, error) {
	db.DPrintf(db.WATCH, "NewDirWatcher: Creating watch on %s", pn)

	fd, err := fslib.Open(pn, sp.OREAD)
	if err != nil {
		return nil, err
	}

	watchFd, err := fslib.DirWatch(fd)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.WATCH, "NewDirWatcher: Created watch on %s with fd=%d", pn, watchFd)

	ch := make(chan *protsrv_proto.WatchEvent)

	reader := watchReader {
		fslib,
		watchFd,
	}

	bufferedReader := bufio.NewReader(reader)

	dr := &DirWatcher{
		FsLib:   fslib,
		ch:      ch,
		pn:      pn,
		closed:  atomic.Bool{},
		reader:  bufferedReader,
		watchFd: watchFd,
	}

	go dr.readUpdatesIntoChannel()

	return dr, nil
}

func NewDirWatcherWithRead(fslib *fslib.FsLib, pn string) ([]string, *DirWatcher, error) {
	var ents []string
	var dw *DirWatcher

	for {
		sts, _, err := fslib.ReadDir(pn)
		if err != nil {
			return nil, nil, err
		}

		dw, err = NewDirWatcher(fslib, pn)
		if err != nil {
			if serr.IsErrCode(err, serr.TErrVersion) {
				continue
			} else {
				return nil, nil, err
			}
		}

		for _, st := range sts {
			ents = append(ents, st.Name)
		}
		break
	}

	return ents, dw, nil
}

func (dr *DirWatcher) readUpdatesIntoChannel() {
	for !dr.closed.Load() {
		eventList, err := dr.readUpdates()

		if err != nil {
			close(dr.ch)
			if serr.IsErrCode(err, serr.TErrClosed) {
				db.DPrintf(db.WATCH, "DirWatcher readUpdatesIntoChannel: watch stream for %s closed %v", dr.pn, err)
				return
			} else {
				db.DFatalf("DirWatcher readUpdatesIntoChannel: failed to read updates %v", err)
				return
			}
		}

		for _, event := range eventList.Events {
			dr.ch <- event
		}
	}

	close(dr.ch)
}

func (dr *DirWatcher) isWatchClosed(err error) bool {
	if err == nil {
		return false
	}

	return dr.closed.Load() || serr.IsErrCode(err, serr.TErrClosed) || serr.IsErrCode(err, serr.TErrUnreachable) || serr.IsErrCode(err, serr.TErrUnknownfid) || err == io.ErrUnexpectedEOF
}

func (dr *DirWatcher) readUpdates() (*protsrv_proto.WatchEventList, error) {
	eventList := &protsrv_proto.WatchEventList{}

	var length uint32
	err := binary.Read(dr.reader, binary.LittleEndian, &length)
	if dr.isWatchClosed(err) {
		db.DPrintf(db.WATCH, "DirWatcher ReadUpdates: watch stream for %s closed %v", dr.pn, err)
		return eventList, serr.NewErr(serr.TErrClosed, "")
	}
	if err != nil {
		db.DFatalf("failed to read length %v", err)
	}
	data := make([]byte, length)
	numRead, err := io.ReadFull(dr.reader, data)
	if dr.isWatchClosed(err) {
		db.DPrintf(db.WATCH, "DirWatcher ReadUpdates: watch stream for %s closed %v", dr.pn, err)
		return eventList, serr.NewErr(serr.TErrClosed, "")
	}
	if err != nil {
		db.DFatalf("watch stream produced err %v", err)
	}

	if uint32(numRead) != length {
		db.DFatalf("only received %d bytes, expected %d bytes", numRead, length)
	}

	err = proto.Unmarshal(data, eventList)
	if err != nil {
		db.DFatalf("DirWatcher: failed to unmarshal data %v", err)
	}
	db.DPrintf(db.WATCH, "DirWatcher ReadUpdates: received %d bytes with %d events", numRead, len(eventList.Events))

	return eventList, nil
}

func (dr *DirWatcher) Close() error {
	db.DPrintf(db.WATCH, "DirWatcher: closing watch on %s", dr.pn)
	dr.closed.Store(true)
	return dr.CloseFd(dr.watchFd)
}

func (dr *DirWatcher) Events() <-chan *protsrv_proto.WatchEvent {
	return dr.ch
}
