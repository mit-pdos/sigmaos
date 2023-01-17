package uprocsrv

import (
	"os"
	"path"
	// "sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
}

func RunUprocSrv(realm string) error {
	ups := &UprocSrv{}
	pn := path.Join(sp.PROCD, "~local", sp.UPROCDREL)
	db.DPrintf(db.UPROCSRV, "%v: Run %v %s\n", proc.GetName(), pn, os.Environ())
	pds, err := protdevsrv.MakeProtDevSrv(pn, ups)
	if err != nil {
		return err
	}
	db.DPrintf(db.UPROCSRV, "%v: makesrv ok\n", proc.GetName())
	fsl := pds.MemFs.FsLib()
	sts, err := fsl.GetDir(pn + "/")
	if err != nil {
		return err
	}
	db.DPrintf(db.UPROCSRV, "names %v: %v\n", pn, sp.Names(sts))
	return pds.RunServer()
}

func (ups *UprocSrv) Run(req proto.RunRequest, res *proto.RunResponse) error {
	uproc := proc.MakeProcFromProto(req.ProcProto)
	return container.Pexec(uproc)
}
