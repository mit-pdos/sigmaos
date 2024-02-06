// The demux package multiplexes calls over a transport (e.g., TCP
// connection, unix socket, etc.), and matches responses with requests
// using the call's tag.
package demux

import (
	"bufio"
	"sync/atomic"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type DemuxClntI interface {
	ReportError(err error)
}

type WriteCallF func(*bufio.Writer, CallI) *serr.Err

type DemuxClnt struct {
	out     *bufio.Writer
	in      *bufio.Reader
	callmap *callMap
	calls   chan CallI
	clnti   DemuxClntI
	nwriter *atomic.Int64
}

type reply struct {
	rep CallI
	err *serr.Err
}

func NewDemuxClnt(out *bufio.Writer, in *bufio.Reader, rf ReadCallF, wf WriteCallF, clnti DemuxClntI) *DemuxClnt {
	dmx := &DemuxClnt{
		out:     out,
		in:      in,
		callmap: newCallMap(),
		calls:   make(chan CallI),
		clnti:   clnti,
		nwriter: new(atomic.Int64),
	}
	go dmx.reader(rf)
	go dmx.writer(wf)
	return dmx
}

func (dmx *DemuxClnt) writer(wf WriteCallF) {
	for {
		req, ok := <-dmx.calls
		if !ok {
			db.DPrintf(db.DEMUXCLNT, "writer: replies closed\n")
			return
		}

		// In error cases, drain calls until SendReceive calls close
		// on calls

		if dmx.IsClosed() {
			continue
		}
		if err := wf(dmx.out, req); err != nil {
			db.DPrintf(db.DEMUXCLNT, "wf req %v error %v\n", req, err)
			continue
		}
		if err := dmx.out.Flush(); err != nil {
			db.DPrintf(db.DEMUXCLNT, "Flush req %v err %v\n", req, err)
			continue
		}
	}
}

func (dmx *DemuxClnt) reply(tag sessp.Ttag, rep CallI, err *serr.Err) {
	if ch, ok := dmx.callmap.remove(tag); ok {
		ch <- reply{rep, err}
	} else {
		db.DFatalf("reply remove missing %v\n", tag)
	}
}

func (dmx *DemuxClnt) reader(rf ReadCallF) {
	for {
		c, err := rf(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXCLNT, "reader rf err %v\n", err)
			dmx.callmap.close()
			break
		}
		dmx.reply(c.Tag(), c, nil)
	}
	for _, t := range dmx.callmap.outstanding() {
		db.DPrintf(db.DEMUXCLNT, "reader fail %v\n", t)
		dmx.reply(t, nil, serr.NewErr(serr.TErrUnreachable, "reader"))
	}

}

func (dmx *DemuxClnt) SendReceive(req CallI) (CallI, *serr.Err) {
	ch := make(chan reply)
	if err := dmx.callmap.put(req.Tag(), ch); err != nil {
		db.DPrintf(db.DEMUXCLNT, "SendReceive: enqueue req %v err %v\n", req, err)
		return nil, err
	}
	dmx.nwriter.Add(1)
	dmx.calls <- req
	rep := <-ch
	if dmx.nwriter.Add(-1) == 0 && dmx.callmap.isClosed() {
		close(dmx.calls)
	}
	return rep.rep, rep.err
}

func (dmx *DemuxClnt) Close() error {
	return dmx.callmap.close()
}

func (dmx *DemuxClnt) IsClosed() bool {
	return dmx.callmap.isClosed()
}
