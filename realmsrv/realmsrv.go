package realmsrv

import (
	"os"
	// "sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevsrv"
	"sigmaos/realmsrv/proto"
	sp "sigmaos/sigmap"
)

type RealmSrv struct {
	fsl   *fslib.FsLib
	pclnt *procclnt.ProcClnt
	ch    chan struct{}
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
	rs.fsl = pds.MemFs.FsLib()
	rs.pclnt = pds.MemFs.ProcClnt()
	sts, err := rs.fsl.GetDir(sp.REALMD + "/")
	if err != nil {
		return err
	}
	db.DPrintf(db.REALMD, "names %v: %v\n", sp.REALMD, sp.Names(sts))
	err = pds.RunServer()
	return nil
}

func (rm *RealmSrv) Make(req proto.MakeRequest, res *proto.MakeResult) error {
	db.DPrintf(db.REALMD, "RealmSrv.Make %v\n", req.Realm)

	p := proc.MakeProc("named", []string{":1111", "testrealm"})
	if err := rm.pclnt.Spawn(p); err != nil {
		return err
	}
	if err := rm.pclnt.WaitStart(p.GetPid()); err != nil {
		return err
	}
	return nil
}
