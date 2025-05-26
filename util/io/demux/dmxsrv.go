package demux

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
)

type CallI interface {
	Tag() sessp.Ttag
}

type TransportI interface {
	ReadCall() (CallI, error)
	WriteCall(CallI) error
	Close() error
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
		if dmx.IsClosed() {
			break
		}
		c, err := dmx.trans.ReadCall()
		if err != nil {
			db.DPrintf(db.DEMUXSRV_ERR, "reader: %p ReadCall err %v\n", dmx, err)
			dmx.srv.ReportError(err)
			break
		}
		go func(c CallI) {
			rep, sr := dmx.srv.ServeRequest(c)
			if sr != nil {
				return
			}
			var err error
			dmx.mu.Lock()
			if !dmx.closed {
				err = dmx.trans.WriteCall(rep)
			}
			dmx.mu.Unlock()
			if err != nil {
				db.DPrintf(db.DEMUXSRV_ERR, "reader: %p WriteCall reply %v error %v\n", dmx, rep, err)
			}
		}(c)
	}
}

func (dmx *DemuxSrv) Close() error {
	dmx.mu.Lock()
	defer dmx.mu.Unlock()

	db.DPrintf(db.DEMUXSRV, "Close dmx %p %d closed? %t\n", dmx, dmx.nreq, dmx.closed)
	if dmx.closed {
		return nil
	}
	dmx.closed = true
	if err := dmx.trans.Close(); err != nil {
		db.DPrintf(db.DEMUXSRV_ERR, "Close trans dmx %p err %v", dmx, err)
	}
	return nil
}

func (dmx *DemuxSrv) IsClosed() bool {
	dmx.mu.Lock()
	defer dmx.mu.Unlock()

	return dmx.closed
}
