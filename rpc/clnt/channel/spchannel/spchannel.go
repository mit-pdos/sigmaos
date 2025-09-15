// Implements an RPC channel abstraction on top of the SigmaOS FsLib API.
package spchannel

import (
	"path/filepath"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/rpc"
	"sigmaos/rpc/clnt/channel"
	rpcdevclnt "sigmaos/rpc/dev/clnt"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type SPChannel struct {
	mu       sync.Mutex
	fsl      *fslib.FsLib
	fd       int
	pn       string
	ep       *sp.Tendpoint
	initDone bool
	initErr  error
}

func NewSPChannelEndpoint(fsl *fslib.FsLib, pn string, ep *sp.Tendpoint, lazyInit bool) (channel.RPCChannel, error) {
	return newSPChannelLazyInit(fsl, pn, ep, lazyInit)
}

func NewSPChannel(fsl *fslib.FsLib, pn string, lazyInit bool) (channel.RPCChannel, error) {
	return NewSPChannelEndpoint(fsl, pn, nil, lazyInit)
}

func newSPChannelLazyInit(fsl *fslib.FsLib, pn string, ep *sp.Tendpoint, lazyInit bool) (channel.RPCChannel, error) {
	ch := &SPChannel{
		fsl: fsl,
		pn:  pn,
		ep:  ep,
	}
	// If eagerly initializing the channel, initialize now and return the result
	if !lazyInit {
		if err := ch.init(); err != nil {
			return nil, err
		}
	}
	return ch, nil
}

func (ch *SPChannel) init() error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// May be called multiple times by multiple threads, so bail out early if
	// init was already called
	if ch.initDone {
		return ch.initErr
	}
	// Note that the channel has been initialized
	ch.initDone = true

	// If endpoint was set, mount it to speed up channel setup
	if ch.ep != nil {
		if err := ch.fsl.MountTree(ch.ep, rpc.RPC, filepath.Join(ch.pn, rpc.RPC)); err != nil {
			ch.initErr = err
			return err
		}
	}

	s := time.Now()
	defer func(s time.Time) {
		db.DPrintf(db.ATTACH_LAT, "NewSigmaPRPCChannel E2e %q lat %v", ch.pn, time.Since(s))
	}(s)
	pn0 := filepath.Join(ch.pn, rpc.RPC)
	s = time.Now()
	sdc, err := rpcdevclnt.NewSessDevClnt(ch.fsl, pn0)
	if err != nil {
		ch.initErr = err
		return err
	}
	db.DPrintf(db.ATTACH_LAT, "NewSigmaPRPCChannel NewSessDevClnt %q lat %v", ch.pn, time.Since(s))
	db.DPrintf(db.RPCCLNT, "Open %v", sdc.DataPn())
	s = time.Now()
	fd, err := ch.fsl.Open(sdc.DataPn(), sp.ORDWR)
	if err != nil {
		ch.initErr = err
		return err
	}
	db.DPrintf(db.ATTACH_LAT, "NewSigmaPRPCChannel Open %q lat %v", ch.pn, time.Since(s))
	ch.fd = fd
	ch.initErr = nil
	return nil
}

func (ch *SPChannel) checkInit() error {
	// Fast path: just check the bool value without holding the lock
	if ch.initDone {
		return nil
	}
	return ch.init()
}

func (ch *SPChannel) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	ch.checkInit()
	return ch.fsl.WriteRead(ch.fd, iniov, outiov)
}

func (ch *SPChannel) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	ch.checkInit()
	return ch.fsl.ReadRPCStats(ch.pn)
}
