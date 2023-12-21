// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"os"
	"os/exec"

	"sigmaos/demux"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	"sigmaos/sessp"
	// sp "sigmaos/sigmap"
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
	cmd := exec.Command("../bin/linux/sigmaclntd", []string{}...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	dmx := demux.NewDemuxClnt(stdin, stdout)
	scc := &SigmaClntClnt{dmx, nil, 0, cmd}
	scc.rpcc = rpcclnt.NewRPCClntCh(scc)
	return scc, nil
}

func (scc *SigmaClntClnt) Shutdown() error {
	return scc.cmd.Process.Kill()
}
