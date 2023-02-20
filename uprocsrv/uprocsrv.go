package uprocsrv

import (
	"os"
	"path"
	// "sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	ch chan struct{}
}

func RunUprocSrv(realm string, ptype proc.Ttype) error {
	ups := &UprocSrv{}
	ups.ch = make(chan struct{})
	pn := path.Join(sp.SCHEDD, "~local", sp.UPROCDREL, realm, ptype.String())
	db.DPrintf(db.UPROCD, "%v: Run %v %s\n", proc.GetName(), pn, os.Environ())
	pds, err := protdevsrv.MakeProtDevSrv(pn, ups)
	if err != nil {
		return err
	}
	if err := container.SetupIsolationEnv(); err != nil {
		db.DFatalf("Error setting up isolation env: %v", err)
	}
	err = pds.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.MakeProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Get uproc %v", uproc)
	return container.RunUProc(uproc)
}
