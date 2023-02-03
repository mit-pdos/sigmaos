package uprocsrv

import (
	"os"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	// sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	ch chan struct{}
}

func RunUprocSrv(realm string, ptype proc.Ttype) error {
	ups := &UprocSrv{}
	ups.ch = make(chan struct{})
	db.DPrintf(db.UPROCD, "%v: Run %v %s\n", proc.GetName(), realm, os.Environ())
	pds, err := protdevsrv.MakeProtDevSrvPort("", container.PORT, ups)
	if err != nil {
		return err
	}
	err = pds.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func (ups *UprocSrv) Run(req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.MakeProcFromProto(req.ProcProto)
	return container.RunUProc(uproc)
}
