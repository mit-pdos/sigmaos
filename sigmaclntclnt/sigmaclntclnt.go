// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"io"

	// db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	// sp "sigmaos/sigmap"
)

type SigmaClntClnt struct {
	req  io.Writer
	rep  io.Reader
	rpcc *rpcclnt.RPCClnt
}

func (scc *SigmaClntClnt) WriteRead(a []byte) ([]byte, error) {
	if err := frame.WriteFrame(scc.req, a); err != nil {
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
