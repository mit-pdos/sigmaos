// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"io"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	// "sigmaos/serr"
	scproto "sigmaos/sigmaclntsrv/proto"
	sp "sigmaos/sigmap"
)

type SigmaClntClnt struct {
	req  io.Writer
	rep  io.Reader
	rpcc *rpcclnt.RPCClnt
}

func (scc *SigmaClntClnt) WriteRead(a []byte) ([]byte, error) {
	if err := frame.WriteFrame(scc.req, a); err != nil {
		db.DPrintf(db.ALWAYS, "WriteFrame err %v\n", err)
		return nil, err
	}
	b, r := frame.ReadFrame(scc.rep)
	if r != nil {
		return nil, r
	}
	return b, nil
}

func (scc *SigmaClntClnt) StatsSrv() (*rpc.SigmaRPCStats, error) {
	return nil, nil
}

func NewSigmaClntClnt(req io.Writer, rep io.Reader) *SigmaClntClnt {
	scc := &SigmaClntClnt{req, rep, nil}
	scc.rpcc = rpcclnt.NewRPCClntCh(scc)
	return scc
}

func (scc *SigmaClntClnt) Stat(path string) (*sp.Stat, error) {
	req := scproto.StatRequest{Path: path}
	rep := scproto.StatReply{}
	err := scc.rpcc.RPC("SigmaClntSrv.Stat", &req, &rep)
	if err != nil {
		return nil, err
	}
	return rep.Stat, nil
}
