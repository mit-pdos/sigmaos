package demux

import (
	"bufio"
	"sync"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type DemuxI interface {
	ServeRequest([]byte) ([]byte, *serr.Err)
}

type reply struct {
	data  []byte
	seqno sessp.Tseqno
	err   *serr.Err
}

// DemuxSrv demultiplexes RPCs from a single transport and multiplexes
// the reponses on the transport.
type DemuxSrv struct {
	mu      sync.Mutex
	in      *bufio.Reader
	out     *bufio.Writer
	serve   DemuxI
	replies chan reply
	closed  bool
	nreq    int
}

func NewDemuxSrv(in *bufio.Reader, out *bufio.Writer, serve DemuxI) *DemuxSrv {
	dmx := &DemuxSrv{in: in, out: out, serve: serve, replies: make(chan reply)}
	go dmx.reader()
	go dmx.writer()
	return dmx
}

func (dmx *DemuxSrv) reader() {
	for {
		seqno, err := frame.ReadSeqno(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXSRV, "reader: ReadSeqno err %v\n", err)
			break
		}
		request, err := frame.ReadFrame(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXSRV, "reader: ReadFrame err %v\n", err)
			break
		}
		go func(r []byte, s sessp.Tseqno) {
			if !dmx.IncNreq() { // handle req?
				return
			}
			db.DPrintf(db.DEMUXSRV, "reader: serve %v\n", s)
			rep, err := dmx.serve.ServeRequest(r)
			db.DPrintf(db.DEMUXSRV, "reader: reply %v %v\n", s, err)
			dmx.replies <- reply{rep, s, err}
			if dmx.DecNreq() {
				close(dmx.replies)
			}
		}(request, seqno)
	}
}

func (dmx *DemuxSrv) writer() {
	for {
		reply, ok := <-dmx.replies
		if !ok {
			db.DPrintf(db.DEMUXSRV, "%v writer: replies closed\n")
			return
		}
		if reply.err != nil {
			// XXX send err back to who created DemuxSrv?
			db.DFatalf("writer: ServeRequest internal err %v\n", reply.err)
			continue
		}
		if err := frame.WriteSeqno(reply.seqno, dmx.out); err != nil {
			db.DPrintf(db.DEMUXSRV, "writer: WriteSeqno err %v\n", err)
			continue
		}
		if err := frame.WriteFrame(dmx.out, reply.data); err != nil {
			db.DPrintf(db.DEMUXSRV, "writer: WriteFrame err %v\n", err)
			continue
		}
		if error := dmx.out.Flush(); error != nil {
			db.DPrintf(db.DEMUXSRV, "Flush error %v\n", error)
		}
	}
}

func (dmx *DemuxSrv) Close() error {
	dmx.mu.Lock()
	defer dmx.mu.Unlock()
	dmx.closed = true
	return nil
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
