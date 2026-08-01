// Implements an RPC channel abstraction on top of the SigmaOS FsLib API.
package spchannel

import (
	"path/filepath"
	"sync"
	"sync/atomic"
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
	mu  sync.Mutex
	fsl *fslib.FsLib
	fd  int
	pn  string
	ep  *sp.Tendpoint
	// Set only once init has finished, so that a concurrent caller taking the
	// lock-free fast path in checkInit never observes "initialized" while fd is
	// still unset — which would send an RPC on fd 0 and be rejected as an
	// unknown fid. Atomic because that fast path reads it without the lock;
	// initErr and fd are written before it is set and read only after, so the
	// store/load pair orders them too.
	initDone atomic.Bool
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

func (ch *SPChannel) init() (err error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// May be called multiple times by multiple threads, so bail out early if
	// init was already called
	if ch.initDone.Load() {
		return ch.initErr
	}
	// Publish the outcome once, on the way out: attempted exactly once (a
	// failure is remembered rather than retried), and only visible to the fast
	// path after fd has been set.
	defer func() {
		ch.initErr = err
		ch.initDone.Store(true)
	}()

	// If endpoint was set, mount it to speed up channel setup
	if ch.ep != nil {
		if err := ch.fsl.MountTree(ch.ep, rpc.RPC, filepath.Join(ch.pn, rpc.RPC)); err != nil {
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
		return err
	}
	db.DPrintf(db.ATTACH_LAT, "NewSigmaPRPCChannel NewSessDevClnt %q lat %v", ch.pn, time.Since(s))
	db.DPrintf(db.RPCCLNT, "Open %v", sdc.DataPn())
	s = time.Now()
	fd, err := ch.fsl.Open(sdc.DataPn(), sp.ORDWR)
	if err != nil {
		return err
	}
	db.DPrintf(db.ATTACH_LAT, "NewSigmaPRPCChannel Open %q lat %v", ch.pn, time.Since(s))
	ch.fd = fd
	return nil
}

func (ch *SPChannel) checkInit() error {
	// Fast path: just check the flag without holding the lock
	if ch.initDone.Load() {
		return ch.initErr
	}
	return ch.init()
}

func (ch *SPChannel) SendReceive(iniov *sessp.IoVec, outiov *sessp.IoVec) error {
	// Propagate the init error rather than sending on an unset fd.
	if err := ch.checkInit(); err != nil {
		return err
	}
	return ch.fsl.WriteRead(ch.fd, iniov, outiov)
}

func (ch *SPChannel) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	if err := ch.checkInit(); err != nil {
		return nil, err
	}
	return ch.fsl.ReadRPCStats(ch.pn)
}
