package demux

import (
	"bufio"

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
}

// DemuxSrv demultiplexes RPCs from a single transport and multiplexes
// the reponses on the transport.
type DemuxSrv struct {
	in      *bufio.Reader
	out     *bufio.Writer
	serve   DemuxI
	replies chan reply
}

func NewDemuxSrv(in *bufio.Reader, out *bufio.Writer, serve DemuxI) *DemuxSrv {
	dmx := &DemuxSrv{in, out, serve, make(chan reply)}
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
			db.DPrintf(db.DEMUXSRV, "reader: serve %v\n", s)
			rep, err := dmx.serve.ServeRequest(r)
			if err != nil {
				db.DPrintf(db.DEMUXSRV, "reader: ServeRequest err %v\n", err)
				return
			}
			db.DPrintf(db.DEMUXSRV, "reader: reply %v\n", s)
			dmx.replies <- reply{rep, s}
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
		if err := frame.WriteSeqno(reply.seqno, dmx.out); err != nil {
			db.DPrintf(db.DEMUXSRV, "writer: WriteSeqno err %v\n", err)
			break
		}
		if err := frame.WriteFrame(dmx.out, reply.data); err != nil {
			db.DPrintf(db.DEMUXSRV, "writer: WriteFrame err %v\n", err)
			break
		}
		if error := dmx.out.Flush(); error != nil {
			db.DPrintf(db.DEMUXSRV, "Flush error %v\n", error)
		}

	}
}
