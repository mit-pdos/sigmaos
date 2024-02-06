package demux

import (
	"bufio"
	"io"
	"sync"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type CallI interface {
	Tag() sessp.Ttag
}

type ReadCallF func(io.Reader) (CallI, *serr.Err)

type DemuxSrvI interface {
	ServeRequest(CallI) (CallI, *serr.Err)
	ReportError(err error)
}

// DemuxSrv demultiplexes RPCs from a single transport and multiplexes
// the reponses on the transport.
type DemuxSrv struct {
	mu      sync.Mutex
	in      *bufio.Reader
	out     *bufio.Writer
	serve   DemuxSrvI
	replies chan reply
	closed  bool
	nreq    int
}

func NewDemuxSrv(in *bufio.Reader, out *bufio.Writer, rf ReadCallF, wf WriteCallF, serve DemuxSrvI) *DemuxSrv {
	dmx := &DemuxSrv{in: in, out: out, serve: serve, replies: make(chan reply)}
	go dmx.reader(rf)
	go dmx.writer(wf)
	return dmx
}

func (dmx *DemuxSrv) reader(rf ReadCallF) {
	for {
		c, err := rf(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXSRV, "reader: rf err %v\n", err)
			dmx.serve.ReportError(err)
			break
		}
		go func(c CallI) {
			if !dmx.IncNreq() { // handle req?
				return // done
			}

			rep, err := dmx.serve.ServeRequest(c)
			dmx.replies <- reply{rep, err}
			if dmx.DecNreq() {
				close(dmx.replies)
			}
		}(c)
	}
}

func (dmx *DemuxSrv) writer(wf WriteCallF) {
	for {
		reply, ok := <-dmx.replies
		if !ok {
			db.DPrintf(db.DEMUXSRV, "writer: replies closed\n")
			return
		}

		// In error cases drain replies until reader closes replies

		if reply.err != nil {
			continue
		}
		if err := wf(dmx.out, reply.rep); err != nil {
			db.DPrintf(db.DEMUXSRV, "wf reply %v error %v\n", reply, err)
			continue
		}
		if err := dmx.out.Flush(); err != nil {
			db.DPrintf(db.DEMUXSRV, "Flush reply %v err %v\n", reply, err)
			continue
		}
	}
}

func (dmx *DemuxSrv) Close() error {
	dmx.mu.Lock()
	defer dmx.mu.Unlock()
	dmx.closed = true
	db.DPrintf(db.DEMUXSRV, "Close %d\n", dmx.nreq)
	return nil
}

func (dmx *DemuxSrv) IsClosed() bool {
	dmx.mu.Lock()
	defer dmx.mu.Unlock()
	return dmx.closed
}

func (dmx *DemuxSrv) IncNreq() bool {
	dmx.mu.Lock()
	defer dmx.mu.Unlock()

	if dmx.closed {
		return false
	}
	dmx.nreq++
	return true
}

func (dmx *DemuxSrv) DecNreq() bool {
	dmx.mu.Lock()
	defer dmx.mu.Unlock()

	dmx.nreq--
	if dmx.nreq == 0 && dmx.closed {
		return true
	}
	return false
}
