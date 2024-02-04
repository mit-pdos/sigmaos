package demux

import (
	"bufio"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type DemuxClntI interface {
	ReportError(err error)
}

// DemuxClnt multiplexes calls (a request/reply pair) on a single
// transport and demultiplexes responses.
type DemuxClnt struct {
	tag     sessp.Tseqno
	out     *bufio.Writer
	in      *bufio.Reader
	callmap *callMap
	calls   chan *call
	clnti   DemuxClntI
}

func NewDemuxClnt(out *bufio.Writer, in *bufio.Reader, nframe int, clnti DemuxClntI) *DemuxClnt {
	dmx := &DemuxClnt{
		out:     out,
		in:      in,
		callmap: newCallMap(),
		calls:   make(chan *call),
		clnti:   clnti,
	}
	go dmx.reader(nframe)
	go dmx.writer()
	return dmx
}

func (dmx *DemuxClnt) NextTag() sessp.Ttag {
	seqp := &dmx.tag
	s := seqp.Next()
	return sessp.Ttag(s)
}

func (dmx *DemuxClnt) writer() {
	for {
		call, ok := <-dmx.calls
		if !ok {
			db.DPrintf(db.DEMUXCLNT, "writer: replies closed\n")
			return
		}
		if err := frame.WriteTagFrames(call.request, call.tag, dmx.out); err != nil {
			db.DPrintf(db.DEMUXCLNT, "WriteTagFrames err %v\n", err)
			dmx.reply(call.tag, nil, serr.NewErr(serr.TErrUnreachable, err.Error()))
			break
		}
		if error := dmx.out.Flush(); error != nil {
			db.DPrintf(db.DEMUXCLNT, "Flush error %v\n", error)
			dmx.reply(call.tag, nil, serr.NewErr(serr.TErrUnreachable, error.Error()))
		}
	}
}

func (dmx *DemuxClnt) reply(tag sessp.Ttag, reply []frame.Tframe, err *serr.Err) {
	call, last := dmx.callmap.remove(tag)
	if call == nil {
		db.DFatalf("Remove err %v\n", tag)
	}
	if last {
		close(dmx.calls)
	}
	call.reply = reply
	call.ch <- err
}

func (dmx *DemuxClnt) reader(nframe int) {
	for {
		tag, err := frame.ReadTag(dmx.in)
		if err != nil {
			dmx.clnti.ReportError(err)
			break
		}
		reply, err := frame.ReadFrames(dmx.in, nframe)
		if err != nil {
			dmx.clnti.ReportError(err)
			break
		}
		db.DPrintf(db.DEMUXCLNT, "reader: reply %v\n", tag)
		dmx.reply(tag, reply, nil)
	}
	for _, s := range dmx.callmap.outstanding() {
		dmx.reply(s, nil, serr.NewErr(serr.TErrUnreachable, "dmxclnt"))
	}
}

func (dmx *DemuxClnt) SendReceive(a []frame.Tframe) ([]frame.Tframe, *serr.Err) {
	t := dmx.NextTag()
	call := &call{request: a, tag: t, ch: make(chan *serr.Err)}
	if err := dmx.callmap.put(t, call); err != nil {
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
