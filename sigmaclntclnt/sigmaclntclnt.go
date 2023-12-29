// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"bufio"
	"net"

	db "sigmaos/debug"
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
	conn  net.Conn
}

func (scc *SigmaClntClnt) SendReceive(a []byte) ([]byte, error) {
	return scc.dmx.SendReceive(a)
}

func (scc *SigmaClntClnt) StatsSrv() (*rpc.SigmaRPCStats, error) {
	return nil, nil
}

func (scc *SigmaClntClnt) ReportError(err error) {
	db.DPrintf(db.DEMUXCLNT, "ReportError %v", err)
	go func() {
		scc.Close()
	}()
}

func NewSigmaClntClnt() (*SigmaClntClnt, error) {
	conn, err := net.Dial("unix", sp.SIGMASOCKET)
	if err != nil {
		return nil, err
	}
	scc := &SigmaClntClnt{nil, nil, 0, conn}
	scc.dmx = demux.NewDemuxClnt(bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN), scc)
	scc.rpcc = rpcclnt.NewRPCClntCh(scc)
	return scc, nil
}

func (scc *SigmaClntClnt) Close() error {
	if err := scc.conn.Close(); err != nil {
		return err
	}
	return scc.dmx.Close()
}
