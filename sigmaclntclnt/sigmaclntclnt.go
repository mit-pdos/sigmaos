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
	scproto "sigmaos/sigmaclntsrv/proto"
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
		scc.close()
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

// Tell sigmaclntd to shut down
func (scc *SigmaClntClnt) Shutdown() error {
	req := scproto.SigmaNullRequest{}
	rep := scproto.SigmaErrReply{}
	err := scc.rpcErr("SigmaClntSrvAPI.Shutdown", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Shutdown %v %v %v", req, rep, err)
	return err
}

// Close the socket connection
func (scc *SigmaClntClnt) close() error {
	if err := scc.conn.Close(); err != nil {
		return err
	}
	return scc.dmx.Close()
}
