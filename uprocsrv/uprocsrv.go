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
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	mu       sync.Mutex
	ch       chan struct{}
	pds      *protdevsrv.ProtDevSrv
	kc       *kernelclnt.KernelClnt
	kernelId string
	net      string
}

func RunUprocSrv(realm, kernelId string, ptype proc.Ttype, up string) error {
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{})}

	ip, _ := container.LocalIP()
	db.DPrintf(db.UPROCD, "%v: Run %v %v %v %s IP %s\n", proc.GetName(), realm, kernelId, up, os.Environ(), ip)

	var pds *protdevsrv.ProtDevSrv
	var err error
	if up == port.NOPORT.String() {
		pn := path.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, realm, ptype.String())
		pds, err = protdevsrv.MakeProtDevSrv(pn, ups)
	} else {
		// The kernel will advertise the server, so pass "" as pn.
		pds, err = protdevsrv.MakeProtDevSrvPort("", up, ups)
	}
	if err != nil {
		return err
	}
	if err := container.SetupIsolationEnv(); err != nil {
		db.DFatalf("Error setting up isolation env: %v", err)
	}
	ups.pds = pds
	ups.net = proc.GetNet()
	err = pds.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.MakeProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Get uproc %v", uproc)
	return container.RunUProc(uproc, ups.kernelId, proc.GetPid(), ups.net)
}
