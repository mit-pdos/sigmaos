// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"io"
	"os"
	"os/exec"

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
	cmd  *exec.Cmd
}

func (scc *SigmaClntClnt) SendReceive(a []byte) ([]byte, error) {
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

	scc := &SigmaClntClnt{stdin, stdout, nil, cmd}
	scc.rpcc = rpcclnt.NewRPCClntCh(scc)
	return scc, nil
}

func (scc *SigmaClntClnt) Shutdown() error {
	return scc.cmd.Process.Kill()
}
