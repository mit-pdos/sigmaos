// Package spproxysrv is an RPC-based server that proxies the
// [sigmaos] interface over a pipe; it reads requests on stdin and
// write responses to stdout.
package srv

import (
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"

	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	dialproxysrv "sigmaos/dialproxy/srv"
	"sigmaos/proc"
	"sigmaos/proxy/sigmap/clnt"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fidclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	GROW_FD = 10000
)

// SPProxySrv maintains the state of the spproxysrv. All
// SigmaSrvClnt's share one fid table
type SPProxySrv struct {
	mu   sync.Mutex
	pe   *proc.ProcEnv
	nps  *dialproxysrv.DialProxySrv
	fidc *fidclnt.FidClnt
	psm  *ProcStateMgr
}

func newSPProxySrv() (*SPProxySrv, error) {
	pe := proc.GetProcEnv()
	nps, err := dialproxysrv.NewDialProxySrv(pe)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewDialProxySrv: %v", err)
		return nil, err
	}
	spps := &SPProxySrv{
		pe:   pe,
		nps:  nps,
		fidc: fidclnt.NewFidClnt(pe, dialproxyclnt.NewDialProxyClnt(pe)),
	}
	spps.psm = NewProcStateMgr(spps)
	db.DPrintf(db.SPPROXYSRV, "newSPProxySrv ProcEnv:%v", pe)
	return spps, nil
}

func (spps *SPProxySrv) runServer() error {
	// Create a socket for uprocd to connect to & control spproxysrv
	ctrlSocket, err := net.Listen("unix", sp.SIGMASOCKET_CTRL)
	if err != nil {
		return err
	}
	if err := os.Chmod(sp.SIGMASOCKET_CTRL, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket_ctrl: %v", err)
	}
	// Serve uprocd control messages
	go func() {
		for {
			conn, err := ctrlSocket.Accept()
			if err != nil {
				db.DPrintf(db.ERROR, "Error accept ctrl socket conn: %v", err)
			}
			newCtrlConn(conn, spps)
		}
	}()
	// Create a socket for proxied procs to connect to
	socket, err := net.Listen("unix", sp.SIGMASOCKET)
	if err != nil {
		return err
	}
	if err := os.Chmod(sp.SIGMASOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.SPPROXYSRV, "runServer: spproxyd listening on %v", sp.SIGMASOCKET)
	if _, err := io.WriteString(os.Stdout, "r"); err != nil {
		db.DFatalf("Err runServer: %v", err)
		return err
	}
	db.DPrintf(db.SPPROXYSRV, "runServer: wrote ready signal")
	if err := syscall.Dup2(1, GROW_FD); err != nil {
		db.DFatalf("Error dup2: %v", err)
	}

	go func() {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(os.Stdin, buf); err != nil {
			db.DPrintf(db.SPPROXYSRV_ERR, "read pipe err %v\n", err)
		}
		db.DPrintf(db.SPPROXYSRV, "exiting")
		os.Remove(sp.SIGMASOCKET)
		os.Remove(sp.SIGMASOCKET_CTRL)
		spps.fidc.Close()
		spps.nps.Shutdown()
		os.Exit(0)
	}()

	// Serve proxied procs
	for {
		conn, err := socket.Accept()
		if err != nil {
			return err
		}
		newSigmaClntConn(spps, conn, spps.pe, spps.fidc)
	}
}

// Allocate a sigmaclnt for a proxied proc, if one doesn't exist already.
// Optionally, return the sigmaclnt (consuming it)
func (spps *SPProxySrv) getSigmaClnt(pe *proc.ProcEnv, p *proc.Proc) (*sigmaclnt.SigmaClnt, *epcacheclnt.EndpointCacheClnt, error) {
	db.DPrintf(db.SPPROXYSRV, "getSigmaClnt %v", pe.GetPID())
	pe.UseSPProxy = false
	pe.UseDialProxy = false

	// Initiate sigmaclnt creation
	ps := spps.psm.AllocProcState(pe, p)
	return ps.GetSigmaClnt()
}

func (spps *SPProxySrv) IncomingProc(p *proc.Proc) {
	db.DPrintf(db.SPPROXYSRV, "Informed of incoming proc %v", p.GetPid())
	p.GetProcEnv().UseSPProxy = false
	p.GetProcEnv().UseDialProxy = false
	spps.psm.AllocProcState(p.GetProcEnv(), p)
}

func (spps *SPProxySrv) WaitBootScriptCompletion(pid sp.Tpid) error {
	db.DPrintf(db.SPPROXYSRV, "[%v] Wait for completion of bootscript", pid)
	err := spps.psm.WaitBootScriptCompletion(pid)
	db.DPrintf(db.SPPROXYSRV, "[%v] Done waiting for completion of bootscript: %v", pid, err)
	return err
}

func (spps *SPProxySrv) ProcDone(p *proc.Proc) {
	db.DPrintf(db.SPPROXYSRV, "Informed proc done %v", p.GetPid())
	spps.mu.Lock()
	defer spps.mu.Unlock()

	spps.psm.DelProcState(p)
}

// The spproxyd process enters here
func RunSPProxySrv() error {
	spps, err := newSPProxySrv()
	if err != nil {
		db.DPrintf(db.SPPROXYSRV, "runServer err %v\n", err)
		return err
	}
	// Perf monitoring
	p, err := perf.NewPerf(spps.pe, perf.SPPROXYSRV)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	if err := spps.runServer(); err != nil {
		db.DPrintf(db.SPPROXYSRV, "runServer err %v\n", err)
		return err
	}
	return nil
}

type SPProxySrvCmd struct {
	p   *proc.Proc
	cmd *exec.Cmd
	out io.WriteCloser
	cc  *clnt.CtrlClnt
}

// Inform spproxysrv that a new proc is incoming, and spproxysrv should start
// to create a sigmaclnt for it.
func (sppsc *SPProxySrvCmd) InformIncomingProc(p *proc.Proc) error {
	return sppsc.cc.InformIncomingProc(p)
}

func (sppsc *SPProxySrvCmd) InformProcDone(p *proc.Proc) error {
	return sppsc.cc.InformProcDone(p)
}

func (sppsc *SPProxySrvCmd) GetProc() *proc.Proc {
	return sppsc.p
}

func (sppsc *SPProxySrvCmd) GetCrashed() bool {
	return false
}

func (sppsc *SPProxySrvCmd) Evict() error {
	// Do nothing
	return nil
}

func (sppsc *SPProxySrvCmd) Wait() error {
	if err := sppsc.Shutdown(); err != nil {
		return err
	}
	return sppsc.cmd.Wait()
}

func (sppsc *SPProxySrvCmd) Kill() error {
	db.DFatalf("Unimplemented")
	return nil
}

func (sppsc *SPProxySrvCmd) SetCPUShares(shares int64) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (sppsc *SPProxySrvCmd) GetCPUUtil() (float64, error) {
	db.DFatalf("Unimplemented")
	return 0, nil
}

func (sppsc *SPProxySrvCmd) Run(how proc.Thow, kernelId string, localIP sp.Tip) error {
	db.DFatalf("Unimplemented")
	return nil
}

// Start the spproxyd process
func ExecSPProxySrv(p *proc.Proc, innerIP sp.Tip, outerIP sp.Tip, procdPid sp.Tpid) (*SPProxySrvCmd, error) {
	p.FinalizeEnv(innerIP, outerIP, procdPid)
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
	cc, err := clnt.NewCtrlClnt()
	if err != nil {
		db.DFatalf("Err new spproxy ctrl clnt: %v", err)
		return nil, err
	}
	return &SPProxySrvCmd{
		p:   p,
		cmd: cmd,
		out: stdin,
		cc:  cc,
	}, nil
}

func (sppsc *SPProxySrvCmd) Shutdown() error {
	if _, err := io.WriteString(sppsc.out, "e"); err != nil {
		return err
	}
	return nil
}
