// Implements an RPC channel abstraction on top of the SigmaOS FsLib API.
package spchannel

import (
	"path/filepath"
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
	fsl *fslib.FsLib
	fd  int
	pn  string
}

func NewSPChannelEndpoint(fsl *fslib.FsLib, pn string, ep *sp.Tendpoint) (channel.RPCChannel, error) {
	if err := fsl.MountTree(ep, rpc.RPC, filepath.Join(pn, rpc.RPC)); err != nil {
		return nil, err
	}
	return NewSPChannel(fsl, pn)
}

func NewSPChannel(fsl *fslib.FsLib, pn string) (channel.RPCChannel, error) {
	s := time.Now()
	defer func(s time.Time) {
		db.DPrintf(db.ATTACH_LAT, "NewSigmaPRPCChannel E2e %q lat %v", pn, time.Since(s))
	}(s)

	pn0 := filepath.Join(pn, rpc.RPC)
	sdc, err := rpcdevclnt.NewSessDevClnt(fsl, pn0)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.RPCCLNT, "Open %v", sdc.DataPn())
	s = time.Now()
	fd, err := fsl.Open(sdc.DataPn(), sp.ORDWR)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.ATTACH_LAT, "NewSigmaPRPCChannel Open %q lat %v", pn, time.Since(s))
	return &SPChannel{
		fsl: fsl,
		fd:  fd,
		pn:  pn,
	}, nil
}

func (ch *SPChannel) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	return ch.fsl.WriteRead(ch.fd, iniov, outiov)
}

func (ch *SPChannel) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	return ch.fsl.ReadRPCStats(ch.pn)
}
