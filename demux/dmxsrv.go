package demux

import (
	"bufio"
	"sync"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type DemuxSrvI interface {
	ServeRequest([]frame.Tframe) ([]frame.Tframe, *serr.Err)
	ReportError(err error)
}

type reply struct {
	data []frame.Tframe
	tag  sessp.Ttag
	err  *serr.Err
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

func NewDemuxSrv(in *bufio.Reader, out *bufio.Writer, nframe int, serve DemuxSrvI) *DemuxSrv {
	dmx := &DemuxSrv{in: in, out: out, serve: serve, replies: make(chan reply)}
	go dmx.reader(nframe)
	go dmx.writer()
	return dmx
}

func (dmx *DemuxSrv) reader(nframe int) {
	for {
		request, tag, err := frame.ReadTagFrames(dmx.in, nframe)
		if err != nil {
			dmx.serve.ReportError(err)
			break
		}
		go func(r []frame.Tframe, tag sessp.Ttag) {
			if !dmx.IncNreq() { // handle req?
				return // done
			}
			db.DPrintf(db.DEMUXSRV, "reader: serve %v\n", tag)
			rep, err := dmx.serve.ServeRequest(r)
			db.DPrintf(db.DEMUXSRV, "reader: reply %v %v\n", tag, err)
			dmx.replies <- reply{rep, tag, err}
			if dmx.DecNreq() {
				close(dmx.replies)
			}
		}(request, tag)
	}
}

func (dmx *DemuxSrv) writer() {
	for {
		reply, ok := <-dmx.replies
		if !ok {
			db.DPrintf(db.DEMUXSRV, "writer: replies closed\n")
			return
		}
		if reply.err != nil {
			dmx.serve.ReportError(reply.err)
			continue
		}
		if err := frame.WriteTagFrames(reply.data, reply.tag, dmx.out); err != nil {
			dmx.serve.ReportError(err)
			continue
		}
		if err := dmx.out.Flush(); err != nil {
			dmx.serve.ReportError(err)
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
