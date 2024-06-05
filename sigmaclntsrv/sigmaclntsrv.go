// Package sigmaclntsrv is an RPC-based server that proxies the
// [sigmaos] interface over a pipe; it reads requests on stdin and
// write responses to stdout.
package sigmaclntsrv

import (
	"io"
	"net"
	"os"
	"os/exec"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/netproxyclnt"
	"sigmaos/netproxysrv"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// SigmaClntSrv maintains the state of the sigmaclntsrv. All
// SigmaSrvClnt's share one fid table
type SigmaClntSrv struct {
	pe   *proc.ProcEnv
	nps  *netproxysrv.NetProxySrv
	fidc *fidclnt.FidClnt
}

func newSigmaClntSrv() (*SigmaClntSrv, error) {
	pe := proc.GetProcEnv()
	nps, err := netproxysrv.NewNetProxySrv(pe)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewNetProxySrv: %v", err)
		return nil, err
	}
	scs := &SigmaClntSrv{
		pe:   pe,
		nps:  nps,
		fidc: fidclnt.NewFidClnt(pe, netproxyclnt.NewNetProxyClnt(pe)),
	}
	db.DPrintf(db.SIGMACLNTSRV, "newSigmaClntSrv ProcEnv:%v", pe)
	return scs, nil
}

func (scs *SigmaClntSrv) runServer() error {
	socket, err := net.Listen("unix", sp.SIGMASOCKET)
	if err != nil {
		return err
	}
	if err := os.Chmod(sp.SIGMASOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.TEST, "runServer: sigmaclntd listening on %v", sp.SIGMASOCKET)
	if _, err := io.WriteString(os.Stdout, "r"); err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(os.Stdin, buf); err != nil {
			db.DPrintf(db.SIGMACLNTSRV_ERR, "read pipe err %v\n", err)
		}
		db.DPrintf(db.SIGMACLNTSRV, "exiting")
		os.Remove(sp.SIGMASOCKET)
		scs.fidc.Close()
		scs.nps.Shutdown()
		os.Exit(0)
	}()

	for {
		conn, err := socket.Accept()
		if err != nil {
			return err
		}
		newSigmaClntConn(conn, scs.pe, scs.fidc)
	}
}

// The sigmaclntd process enter here
func RunSigmaClntSrv() error {
	scs, err := newSigmaClntSrv()
	if err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "runServer err %v\n", err)
		return err
	}
	// Perf monitoring
	p, err := perf.NewPerf(scs.pe, perf.SIGMACLNTSRV)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	if err := scs.runServer(); err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "runServer err %v\n", err)
		return err
	}
	return nil
}

type SigmaClntSrvCmd struct {
	p      *proc.Proc
	cmd    *exec.Cmd
	out    io.WriteCloser
	waited bool
}

type Subsystem interface {
	GetProc() *proc.Proc
	GetHow() proc.Thow
	GetCrashed() bool
	GetContainer() *container.Container
	SetWaited(bool)
	GetWaited() bool
	Wait() error
	Kill() error
	SetCPUShares(shares int64) error
	GetCPUUtil() (float64, error)
	AssignToRealm(realm sp.Trealm, ptype proc.Ttype) error
	GetPortBinding(p sp.Tport) (*port.PortBinding, error)
	Run(how proc.Thow, kernelId string, localIP sp.Tip) error
}

func (scsc *SigmaClntSrvCmd) GetProc() *proc.Proc {
	return scsc.p
}

func (scsc *SigmaClntSrvCmd) GetHow() proc.Thow {
	return proc.HLINUX
}

func (scsc *SigmaClntSrvCmd) GetCrashed() bool {
	return false
}

func (scsc *SigmaClntSrvCmd) GetContainer() *container.Container {
	db.DFatalf("No container")
	return nil
}

func (scsc *SigmaClntSrvCmd) SetWaited(w bool) {
	scsc.waited = w
}

func (scsc *SigmaClntSrvCmd) GetWaited() bool {
	return scsc.waited
}

func (scsc *SigmaClntSrvCmd) Evict() error {
	// Do nothing
	return nil
}

func (scsc *SigmaClntSrvCmd) Wait() error {
	return scsc.Shutdown()
}

func (scsc *SigmaClntSrvCmd) Kill() error {
	db.DFatalf("Unimplemented")
	return nil
}

func (scsc *SigmaClntSrvCmd) SetCPUShares(shares int64) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (scsc *SigmaClntSrvCmd) GetCPUUtil() (float64, error) {
	db.DFatalf("Unimplemented")
	return 0, nil
}

func (scsc *SigmaClntSrvCmd) AssignToRealm(realm sp.Trealm, ptype proc.Ttype) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (scsc *SigmaClntSrvCmd) GetPortBinding(p sp.Tport) (*port.PortBinding, error) {
	db.DFatalf("Unimplemented")
	return nil, nil
}

func (scsc *SigmaClntSrvCmd) Run(how proc.Thow, kernelId string, localIP sp.Tip) error {
	db.DFatalf("Unimplemented")
	return nil
}

// Start the sigmaclntd process
func ExecSigmaClntSrv(p *proc.Proc, innerIP sp.Tip, outerIP sp.Tip, uprocdPid sp.Tpid) (*SigmaClntSrvCmd, error) {
	p.FinalizeEnv(innerIP, outerIP, uprocdPid)
	db.DPrintf(db.SIGMACLNTSRV, "ExecSigmaclntsrv: %v", p)
	cmd := exec.Command("sigmaclntd")
	cmd.Env = p.GetEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "read pipe err %v\n", err)
		return nil, err
	}
	return &SigmaClntSrvCmd{
		p:   p,
		cmd: cmd,
		out: stdin,
	}, nil
}

func (scsc *SigmaClntSrvCmd) Shutdown() error {
	if _, err := io.WriteString(scsc.out, "e"); err != nil {
		return err
	}
	return nil
}
