package demux

import (
	"bufio"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type DemuxClntI interface {
	ReportError(err error)
}

type WriteCallF func(*bufio.Writer, CallI) *serr.Err

// DemuxClnt multiplexes calls (a request/reply pair) on a single
// transport and demultiplexes responses.
type DemuxClnt struct {
	out     *bufio.Writer
	in      *bufio.Reader
	callmap *callMap
	calls   chan *call
	clnti   DemuxClntI
}

func NewDemuxClnt(out *bufio.Writer, in *bufio.Reader, rf ReadCallF, wf WriteCallF, clnti DemuxClntI) *DemuxClnt {
	dmx := &DemuxClnt{
		out:     out,
		in:      in,
		callmap: newCallMap(),
		calls:   make(chan *call),
		clnti:   clnti,
	}
	go dmx.reader(rf)
	go dmx.writer(wf)
	return dmx
}

func (dmx *DemuxClnt) writer(wf WriteCallF) {
	for {
		call, ok := <-dmx.calls
		if !ok {
			db.DPrintf(db.DEMUXCLNT, "writer: replies closed\n")
			return
		}
		if err := wf(dmx.out, call.request); err != nil {
			dmx.reply(call.request.Tag(), nil, serr.NewErr(serr.TErrUnreachable, err.Error()))
			break
		}
		if error := dmx.out.Flush(); error != nil {
			db.DPrintf(db.DEMUXCLNT, "Flush error %v\n", error)
			dmx.reply(call.request.Tag(), nil, serr.NewErr(serr.TErrUnreachable, error.Error()))
		}
	}
}

func (dmx *DemuxClnt) reply(tag sessp.Ttag, reply CallI, err *serr.Err) {
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

func (dmx *DemuxClnt) reader(rf ReadCallF) {
	for {
		c, err := rf(dmx.in)
		if err != nil {
			dmx.callmap.close()
			dmx.clnti.ReportError(err)
			break
		}
		db.DPrintf(db.DEMUXCLNT, "reader: reply %v\n", c)
		dmx.reply(c.Tag(), c, nil)
	}
	for _, c := range dmx.callmap.outstanding() {
		dmx.reply(c.request.Tag(), nil, serr.NewErr(serr.TErrUnreachable, "dmxclnt"))
	}
}

func (dmx *DemuxClnt) SendReceive(req CallI) (CallI, *serr.Err) {
	call := &call{request: req, ch: make(chan *serr.Err)}
	if err := dmx.callmap.put(req.Tag(), call); err != nil {
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
