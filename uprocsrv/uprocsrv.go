package uprocsrv

import (
	"os"
	"sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	mu       sync.Mutex
	ch       chan struct{}
	pds      *protdevsrv.ProtDevSrv
	kc       *kernelclnt.KernelClnt
	kernelId string
}

func RunUprocSrv(realm, kernelId string, ptype proc.Ttype, port string) error {
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{})}

	ip, _ := container.LocalIP()
	db.DPrintf(db.UPROCD, "%v: Run %v %v %v %s IP %s\n", proc.GetName(), realm, kernelId, port, os.Environ(), ip)

	// The kernel will advertise the server, so pass "" as pn.
	pds, err := protdevsrv.MakeProtDevSrvPort("", port, ups)
	if err != nil {
		return err
	}
	ups.pds = pds
	err = pds.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.MakeProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Get uproc %v", uproc)
	return container.RunUProc(uproc, ups.kernelId, proc.GetPid())
}
