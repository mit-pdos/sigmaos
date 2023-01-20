package realmsrv

import (
	"os"
	// "sync"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	"sigmaos/realmsrv/proto"
	sp "sigmaos/sigmap"
)

type RealmSrv struct {
	ch chan struct{}
}

func RunRealmSrv() error {
	rs := &RealmSrv{}
	rs.ch = make(chan struct{})
	db.DPrintf(db.REALMD, "%v: Run %v %s\n", proc.GetName(), sp.REALMD, os.Environ())
	pds, err := protdevsrv.MakeProtDevSrv(sp.REALMD, rs)
	if err != nil {
		return err
	}
	db.DPrintf(db.REALMD, "%v: makesrv ok\n", proc.GetName())
	fsl := pds.MemFs.FsLib()
	sts, err := fsl.GetDir(sp.REALMD + "/")
	if err != nil {
		return err
	}
	db.DPrintf(db.REALMD, "names %v: %v\n", sp.REALMD, sp.Names(sts))
	err = pds.RunServer()
	db.DPrintf(db.REALMD, "RunServer done %v\n", err)
	return nil
}

func (rm *RealmSrv) Make(req proto.MakeRequest, res *proto.MakeResult) error {
	return nil
}
