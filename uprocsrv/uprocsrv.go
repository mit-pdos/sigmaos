package uprocsrv

import (
	"os"
	"sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/kernelclnt"
	kproto "sigmaos/kernelsrv/proto"
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
}

func RunUprocSrv(realm, kernelId string, ptype proc.Ttype) error {
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{})}

	db.DPrintf(db.UPROCD, "%v: Run %v %v %s\n", proc.GetName(), realm, kernelId, os.Environ())

	// The kernel will advertise the server, so pass "" as pn.
	pds, err := protdevsrv.MakeProtDevSrvPort("", container.FPORT.String(), ups)
	if err != nil {
		return err
	}
	ups.pds = pds
	err = pds.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func (ups *UprocSrv) Run(req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.MakeProcFromProto(req.ProcProto)
	return container.RunUProc(uproc)
}

func (ups *UprocSrv) getKernelClnt() (*kernelclnt.KernelClnt, error) {
	ups.mu.Lock()
	defer ups.mu.Unlock()
	if ups.kc == nil {
		db.DPrintf(db.UPROCD, "getKernelClnt kernelId %s\n", ups.kernelId)
		kc, err := kernelclnt.MakeKernelClnt(ups.pds.SigmaClnt().FsLib, sp.BOOT+ups.kernelId)
		if err != nil {
			return nil, err
		}
		ups.kc = kc
	}
	return ups.kc, nil
}

func (ups *UprocSrv) Port(req kproto.PortRequest, res *kproto.PortResult) error {
	kc, err := ups.getKernelClnt()
	if err != nil {
		return err
	}
	hip, pm, err := kc.Port(proc.GetPid(), req.Port)
	if err != nil {
		return err
	}
	db.DPrintf(db.UPROCD, "Ip %s Port pm %v\n", hip, pm)
	res.HostIp = hip
	res.HostPort = pm.HostPort
	res.RealmPort = pm.RealmPort
	return nil
}
