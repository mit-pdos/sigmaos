package demux

import (
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

type DemuxSrv struct {
	mu     sync.Mutex
	in     io.Reader
	out    io.Writer
	serve  DemuxSrvI
	closed bool
	nreq   int
	rf     ReadCallF
	wf     WriteCallF
}

func NewDemuxSrv(in io.Reader, out io.Writer, rf ReadCallF, wf WriteCallF, serve DemuxSrvI) *DemuxSrv {
	dmx := &DemuxSrv{in: in, out: out, serve: serve, wf: wf, rf: rf}
	go dmx.reader()
	return dmx
}

func (dmx *DemuxSrv) reader() {
	for {
		c, err := dmx.rf(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXSRV, "reader: rf err %v\n", err)
			dmx.serve.ReportError(err)
			break
		}
		go func(c CallI) {
			rep, err := dmx.serve.ServeRequest(c)
			if err != nil {
				return
			}
			dmx.mu.Lock()
			err = dmx.wf(dmx.out, rep)
			dmx.mu.Unlock()
			if err != nil {
				db.DPrintf(db.DEMUXSRV, "wf reply %v error %v\n", rep, err)
			}
		}(c)
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
