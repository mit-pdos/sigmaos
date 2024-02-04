package demux

import (
	"bufio"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/sessp"
	// sp "sigmaos/sigmap"
	"sigmaos/serr"
)

type DemuxClntI interface {
	ReportError(err error)
}

// DemuxClnt multiplexes calls (a request/reply pair) on a single
// transport and demultiplexes responses.
type DemuxClnt struct {
	out     *bufio.Writer
	in      *bufio.Reader
	seqno   sessp.Tseqno
	callmap *callMap
	calls   chan *call
	clnti   DemuxClntI
}

func NewDemuxClnt(out *bufio.Writer, in *bufio.Reader, nframe int, clnti DemuxClntI) *DemuxClnt {
	dmx := &DemuxClnt{out, in, 0, newCallMap(), make(chan *call), clnti}
	go dmx.reader(nframe)
	go dmx.writer()
	return dmx
}

func (dmx *DemuxClnt) writer() {
	for {
		call, ok := <-dmx.calls
		if !ok {
			db.DPrintf(db.DEMUXCLNT, "writer: replies closed\n")
			return
		}
		if err := frame.WriteSeqno(call.seqno, dmx.out); err != nil {
			db.DPrintf(db.DEMUXCLNT, "WriteSeqno err %v\n", err)
			dmx.reply(call.seqno, nil, serr.NewErr(serr.TErrUnreachable, err.Error()))
			break
		}
		if err := frame.WriteFrames(call.request, dmx.out); err != nil {
			db.DPrintf(db.DEMUXCLNT, "writeFrames err %v\n", err)
			dmx.reply(call.seqno, nil, serr.NewErr(serr.TErrUnreachable, err.Error()))
			break
		}
		if error := dmx.out.Flush(); error != nil {
			db.DPrintf(db.DEMUXCLNT, "Flush error %v\n", error)
			dmx.reply(call.seqno, nil, serr.NewErr(serr.TErrUnreachable, error.Error()))
		}
	}
}

func (dmx *DemuxClnt) reply(seqno sessp.Tseqno, reply []frame.Tframe, err *serr.Err) {
	call, last := dmx.callmap.remove(seqno)
	if call == nil {
		db.DFatalf("Remove err %v\n", seqno)
	}
	if last {
		close(dmx.calls)
	}
	call.reply = reply
	call.ch <- err
}

func (dmx *DemuxClnt) reader(nframe int) {
	for {
		seqno, err := frame.ReadSeqno(dmx.in)
		if err != nil {
			dmx.clnti.ReportError(err)
			break
		}
		reply, err := frame.ReadFrames(dmx.in, nframe)
		if err != nil {
			dmx.clnti.ReportError(err)
			break
		}
		db.DPrintf(db.DEMUXCLNT, "reader: reply %v\n", seqno)
		dmx.reply(seqno, reply, nil)
	}
	for _, s := range dmx.callmap.outstanding() {
		dmx.reply(s, nil, serr.NewErr(serr.TErrUnreachable, "dmxclnt"))
	}
}

func (dmx *DemuxClnt) SendReceive(a []frame.Tframe) ([]frame.Tframe, *serr.Err) {
	seqp := &dmx.seqno
	s := seqp.Next()
	call := &call{request: a, seqno: s, ch: make(chan *serr.Err)}
	if err := dmx.callmap.put(s, call); err != nil {
		db.DPrintf(db.DEMUXCLNT, "SendReceive: enqueue err %v\n", err)
		return nil, err
	}
	db.DPrintf(db.DEMUXCLNT, "SendReceive: enqueue %v\n", call)
	dmx.calls <- call
	err := <-call.ch
	db.DPrintf(db.DEMUXCLNT, "SendReceive: return %v %v\n", call, err)
	return call.reply, err
}

func (dmx *DemuxClnt) Close() error {
	return dmx.callmap.close()
}

func (dmx *DemuxClnt) IsClosed() bool {
	return dmx.callmap.isClosed()
}
