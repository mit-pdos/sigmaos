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
	"sigmaos/fslib"
	"sigmaos/netsigma"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// SigmaClntSrv maintains the state of the sigmaclntsrv. All
// SigmaSrvClnt's share one fid table
type SigmaClntSrv struct {
	pcfg *proc.ProcEnv
	fidc *fidclnt.FidClnt
}

func newSigmaClntSrv() (*SigmaClntSrv, error) {
	localIP, err := netsigma.LocalIP()
	if err != nil {
		db.DFatalf("Error local IP: %v", err)
	}
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, "127.0.0.1", localIP, "local-build", false, false)
	pcfg.Program = "sigmaclntd"
	pcfg.SetUname("sigmaclntd")
	pcfg.SetPID(sp.GenPid("sigmaclntd"))
	proc.SetSigmaDebugPid(pcfg.GetPID().String())

	scs := &SigmaClntSrv{
		pcfg,
		fidclnt.NewFidClnt(pcfg.Net),
	}
	return scs, nil
}

func (scs *SigmaClntSrv) runServer() error {
	socket, err := net.Listen("unix", sp.SIGMASOCKET)
	if err != nil {
		return err
	}
	db.DPrintf(db.SIGMACLNTSRV, "runServer: listening on %v", sp.SIGMASOCKET)
	if _, err := io.WriteString(os.Stdout, "r"); err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(os.Stdin, buf); err != nil {
			db.DFatalf("read pipe err %v\n", err)
		}
		db.DPrintf(db.SIGMACLNTSRV, "exiting")
		os.Remove(sp.SIGMASOCKET)
		scs.fidc.Close()
		os.Exit(0)
	}()

	for {
		conn, err := socket.Accept()
		if err != nil {
			return err
		}
		newSigmaClntConn(conn, scs.pcfg, scs.fidc)
	}
}

// The sigmaclntd process enter here
func RunSigmaClntSrv(args []string) error {
	scs, err := newSigmaClntSrv()
	if err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "runServer err %v\n", err)
		return err
	}
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
	GetIp(fsl *fslib.FsLib) *sp.Taddr
	AssignToRealm(realm sp.Trealm, ptype proc.Ttype) error
	AllocPort(p sp.Tport) (*port.PortBinding, error)
	Run(how proc.Thow, kernelId string, localIP sp.Thost) error
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

func (scsc *SigmaClntSrvCmd) GetIp(fsl *fslib.FsLib) *sp.Taddr {
	db.DFatalf("Unimplemented")
	return nil
}

func (scsc *SigmaClntSrvCmd) AssignToRealm(realm sp.Trealm, ptype proc.Ttype) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (scsc *SigmaClntSrvCmd) AllocPort(p sp.Tport) (*port.PortBinding, error) {
	db.DFatalf("Unimplemented")
	return nil, nil
}

func (scsc *SigmaClntSrvCmd) Run(how proc.Thow, kernelId string, localIP sp.Thost) error {
	db.DFatalf("Unimplemented")
	return nil
}

// Start the sigmaclntd process
func ExecSigmaClntSrv() (*SigmaClntSrvCmd, error) {
	cmd := exec.Command("sigmaclntd", []string{}...)
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
		p:   proc.NewPrivProcPid(sp.Tpid("sigmaclntd"), "sigmaclntd", nil, true),
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
