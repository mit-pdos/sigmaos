// Package spproxysrv is an RPC-based server that proxies the
// [sigmaos] interface over a pipe; it reads requests on stdin and
// write responses to stdout.
package spproxysrv

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

// SPProxySrv maintains the state of the spproxysrv. All
// SigmaSrvClnt's share one fid table
type SPProxySrv struct {
	pe   *proc.ProcEnv
	nps  *netproxysrv.NetProxySrv
	fidc *fidclnt.FidClnt
}

func newSPProxySrv() (*SPProxySrv, error) {
	pe := proc.GetProcEnv()
	nps, err := netproxysrv.NewNetProxySrv(pe)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewNetProxySrv: %v", err)
		return nil, err
	}
	scs := &SPProxySrv{
		pe:   pe,
		nps:  nps,
		fidc: fidclnt.NewFidClnt(pe, netproxyclnt.NewNetProxyClnt(pe)),
	}
	db.DPrintf(db.SPPROXYSRV, "newSPProxySrv ProcEnv:%v", pe)
	return scs, nil
}

func (scs *SPProxySrv) runServer() error {
	socket, err := net.Listen("unix", sp.SIGMASOCKET)
	if err != nil {
		return err
	}
	if err := os.Chmod(sp.SIGMASOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.TEST, "runServer: spproxyd listening on %v", sp.SIGMASOCKET)
	if _, err := io.WriteString(os.Stdout, "r"); err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(os.Stdin, buf); err != nil {
			db.DPrintf(db.SPPROXYSRV_ERR, "read pipe err %v\n", err)
		}
		db.DPrintf(db.SPPROXYSRV, "exiting")
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

// The spproxyd process enter here
func RunSPProxySrv() error {
	scs, err := newSPProxySrv()
	if err != nil {
		db.DPrintf(db.SPPROXYSRV, "runServer err %v\n", err)
		return err
	}
	// Perf monitoring
	p, err := perf.NewPerf(scs.pe, perf.SPPROXYSRV)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	if err := scs.runServer(); err != nil {
		db.DPrintf(db.SPPROXYSRV, "runServer err %v\n", err)
		return err
	}
	return nil
}

type SPProxySrvCmd struct {
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
	GetPortBinding(p sp.Tport) (*port.PortBinding, error)
	Run(how proc.Thow, kernelId string, localIP sp.Tip) error
}

func (scsc *SPProxySrvCmd) GetProc() *proc.Proc {
	return scsc.p
}

func (scsc *SPProxySrvCmd) GetHow() proc.Thow {
	return proc.HLINUX
}

func (scsc *SPProxySrvCmd) GetCrashed() bool {
	return false
}

func (scsc *SPProxySrvCmd) GetContainer() *container.Container {
	db.DFatalf("No container")
	return nil
}

func (scsc *SPProxySrvCmd) SetWaited(w bool) {
	scsc.waited = w
}

func (scsc *SPProxySrvCmd) GetWaited() bool {
	return scsc.waited
}

func (scsc *SPProxySrvCmd) Evict() error {
	// Do nothing
	return nil
}

func (scsc *SPProxySrvCmd) Wait() error {
	return scsc.Shutdown()
}

func (scsc *SPProxySrvCmd) Kill() error {
	db.DFatalf("Unimplemented")
	return nil
}

func (scsc *SPProxySrvCmd) SetCPUShares(shares int64) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (scsc *SPProxySrvCmd) GetCPUUtil() (float64, error) {
	db.DFatalf("Unimplemented")
	return 0, nil
}

func (scsc *SPProxySrvCmd) GetPortBinding(p sp.Tport) (*port.PortBinding, error) {
	db.DFatalf("Unimplemented")
	return nil, nil
}

func (scsc *SPProxySrvCmd) Run(how proc.Thow, kernelId string, localIP sp.Tip) error {
	db.DFatalf("Unimplemented")
	return nil
}

// Start the spproxyd process
func ExecSPProxySrv(p *proc.Proc, innerIP sp.Tip, outerIP sp.Tip, uprocdPid sp.Tpid) (*SPProxySrvCmd, error) {
	p.FinalizeEnv(innerIP, outerIP, uprocdPid)
	db.DPrintf(db.SPPROXYSRV, "ExecSPProxySrv: %v", p)
	cmd := exec.Command("spproxyd")
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
		db.DPrintf(db.SPPROXYSRV, "read pipe err %v\n", err)
		return nil, err
	}
	return &SPProxySrvCmd{
		p:   p,
		cmd: cmd,
		out: stdin,
	}, nil
}

func (scsc *SPProxySrvCmd) Shutdown() error {
	if _, err := io.WriteString(scsc.out, "e"); err != nil {
		return err
	}
	return nil
}
