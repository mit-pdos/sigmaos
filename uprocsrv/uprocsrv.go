package uprocsrv

import (
	"os"
	"path"
	"sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernelclnt"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	mu       sync.Mutex
	ch       chan struct{}
	pcfg     *proc.ProcEnv
	ssrv     *sigmasrv.SigmaSrv
	kc       *kernelclnt.KernelClnt
	kernelId string
}

func RunUprocSrv(realm, kernelId string, ptype proc.Ttype, up string) error {
	pcfg := proc.GetProcEnv()
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{}), pcfg: pcfg}

	ip, _ := container.LocalIP()
	db.DPrintf(db.UPROCD, "Run %v %v %v %s IP %s", realm, kernelId, up, os.Environ(), ip)

	var ssrv *sigmasrv.SigmaSrv
	var err error
	if up == port.NOPORT.String() {
		pn := path.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, realm, ptype.String())
		ssrv, err = sigmasrv.NewSigmaSrv(pn, ups, pcfg)
	} else {
		// The kernel will advertise the server, so pass "" as pn.
		ssrv, err = sigmasrv.NewSigmaSrvPort("", up, pcfg, ups)
	}
	if err != nil {
		return err
	}
	if err := container.SetupIsolationEnv(); err != nil {
		db.DFatalf("Error setting up isolation env: %v", err)
	}
	ups.ssrv = ssrv
	err = ssrv.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Get uproc %v", uproc)
	return container.RunUProc(uproc)
}
