// Package sigmaclntsrv is an RPC-based server that proxies the
// [sigmaos] interface over a pipe; it reads requests on stdin and
// write responses to stdout.
package sigmaclntsrv

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/keys"
	"sigmaos/netsigma"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// SigmaClntSrv maintains the state of the sigmaclntsrv. All
// SigmaSrvClnt's share one fid table
type SigmaClntSrv struct {
	pe   *proc.ProcEnv
	nps  *netsigma.NetProxySrv
	auth auth.AuthSrv
	fidc *fidclnt.FidClnt
}

// Bootstrap the sigmacltnsrv's ProcEnv by self-signing the token
func bootstrapToken(pe *proc.ProcEnv, pubkey auth.PublicKey, privkey auth.PrivateKey) error {
	kmgr := keys.NewKeyMgr(keys.WithConstGetKeyFn(auth.PublicKey(pubkey)))
	kmgr.AddPrivateKey(sp.Tsigner(pe.GetPID()), privkey)
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf("Error NewAuthSrv: %v", err)
		return err
	}
	if err := as.MintAndSetProcToken(pe); err != nil {
		db.DFatalf("Error MintToken: %v", err)
		return err
	}
	return nil
}

func newSigmaClntSrv(masterPubkey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) (*SigmaClntSrv, error) {
	pe := proc.GetProcEnv()
	if err := bootstrapToken(pe, pubkey, privkey); err != nil {
		db.DFatalf("Error bootstrap token: %v", err)
		return nil, err
	}
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
		return nil, err
	}
	kmgr := keys.NewKeyMgr(keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc))
	// Add the master deployment key
	kmgr.AddPublicKey(auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, masterPubkey)
	// Add this sigmaclntd's keypair to the keymgr
	kmgr.AddPublicKey(sp.Tsigner(pe.GetPID()), pubkey)
	kmgr.AddPrivateKey(sp.Tsigner(pe.GetPID()), privkey)
	db.DPrintf(db.SCHEDD, "kmgr %v", kmgr)
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	nps, err := netsigma.NewNetProxySrv()
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewNetProxySrv: %v", err)
		return nil, err
	}
	scs := &SigmaClntSrv{
		pe:   pe,
		nps:  nps,
		auth: as,
		fidc: fidclnt.NewFidClnt(pe, netsigma.NewNetProxyClnt(pe)),
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
			db.DFatalf("read pipe err %v\n", err)
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
func RunSigmaClntSrv(masterPubkey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) error {
	scs, err := newSigmaClntSrv(masterPubkey, pubkey, privkey)
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
	AllocPort(p sp.Tport) (*port.PortBinding, error)
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

func (scsc *SigmaClntSrvCmd) AllocPort(p sp.Tport) (*port.PortBinding, error) {
	db.DFatalf("Unimplemented")
	return nil, nil
}

func (scsc *SigmaClntSrvCmd) Run(how proc.Thow, kernelId string, localIP sp.Tip) error {
	db.DFatalf("Unimplemented")
	return nil
}

// Start the sigmaclntd process
func ExecSigmaClntSrv(p *proc.Proc, innerIP sp.Tip, outerIP sp.Tip, uprocdPid sp.Tpid, marshaledKeys []string) (*SigmaClntSrvCmd, error) {
	p.FinalizeEnv(innerIP, outerIP, uprocdPid)
	db.DPrintf(db.SIGMACLNTSRV, "ExecSigmaclntsrv: %v", p)
	if len(marshaledKeys) != 3 {
		db.DPrintf(db.ERROR, "Sigmaclntd usage expects bootstrapped keys")
		return nil, fmt.Errorf("Sigmaclntd usage expects bootstrapped keys")
	}
	cmd := exec.Command("sigmaclntd", marshaledKeys...)
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
