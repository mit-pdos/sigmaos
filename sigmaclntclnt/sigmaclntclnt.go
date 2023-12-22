// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"bufio"
	"net"

	"sigmaos/demux"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type SigmaClntClnt struct {
	dmx   *demux.DemuxClnt
	rpcc  *rpcclnt.RPCClnt
	seqno sessp.Tseqno
}

func (scc *SigmaClntClnt) SendReceive(a []byte) ([]byte, error) {
	return scc.dmx.SendReceive(a)
}

func (scc *SigmaClntClnt) StatsSrv() (*rpc.SigmaRPCStats, error) {
	return nil, nil
}

func NewSigmaClntClnt() (*SigmaClntClnt, error) {
	conn, err := net.Dial("unix", sp.SIGMASOCKET)
	if err != nil {
		return nil, err
	}
	dmx := demux.NewDemuxClnt(bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN))
	scc := &SigmaClntClnt{dmx, nil, 0}
	scc.rpcc = rpcclnt.NewRPCClntCh(scc)
	return scc, nil
}
