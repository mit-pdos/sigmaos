// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"bufio"
	"net"
	"os/exec"

	"sigmaos/demux"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	"sigmaos/sessp"
	"sigmaos/sigmaclntsrv"
	sp "sigmaos/sigmap"
)

type SigmaClntClnt struct {
	dmx   *demux.DemuxClnt
	rpcc  *rpcclnt.RPCClnt
	seqno sessp.Tseqno
	cmd   *exec.Cmd
}

func (scc *SigmaClntClnt) SendReceive(a []byte) ([]byte, error) {
	return scc.dmx.SendReceive(a)
}

func (scc *SigmaClntClnt) StatsSrv() (*rpc.SigmaRPCStats, error) {
	return nil, nil
}

func NewSigmaClntClnt() (*SigmaClntClnt, error) {
	cmd, err := sigmaclntsrv.ExecSigmaClntSrv()
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("unix", sigmaclntsrv.SOCKET)
	if err != nil {
		return nil, err
	}
	dmx := demux.NewDemuxClnt(bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN))
	scc := &SigmaClntClnt{dmx, nil, 0, cmd}
	scc.rpcc = rpcclnt.NewRPCClntCh(scc)
	return scc, nil
}

func (scc *SigmaClntClnt) Shutdown() error {
	return scc.cmd.Process.Kill()
}
