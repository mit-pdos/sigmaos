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
type WriteCallF func(io.Writer, CallI) *serr.Err

type TransportI interface {
	ReadCall() (CallI, *serr.Err)
	WriteCall(CallI) *serr.Err
}

type ServerI interface {
	ServeRequest(CallI) (CallI, *serr.Err)
	ReportError(err error)
}

type DemuxSrv struct {
	mu     sync.Mutex
	srv    ServerI
	closed bool
	nreq   int
	trans  TransportI
}

func NewDemuxSrv(srv ServerI, trans TransportI) *DemuxSrv {
	dmx := &DemuxSrv{srv: srv, trans: trans}
	go dmx.reader()
	return dmx
}

func (dmx *DemuxSrv) reader() {
	for {
		c, err := dmx.trans.ReadCall()
		if err != nil {
			db.DPrintf(db.DEMUXSRV, "reader: rf err %v\n", err)
			dmx.srv.ReportError(err)
			break
		}
		go func(c CallI) {
			rep, err := dmx.srv.ServeRequest(c)
			if err != nil {
				return
			}
			dmx.mu.Lock()
			err = dmx.trans.WriteCall(rep)
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
