package dbd

import (
	// db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sessdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

//
// A db proxy exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

const (
	QDEV = "query"
)

func RunDbd(dbdaddr string) error {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	s, err := mkServer(dbdaddr)
	if err != nil {
		return err
	}
	ssrv, err := sigmasrv.MakeSigmaSrv(sp.DB, s, proc.GetProcEnv())
	if err != nil {
		return err
	}
	qd := &queryDev{dbdaddr}
	if _, err := ssrv.Create(QDEV, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := sessdevsrv.MkSessDev(ssrv.MemFs, QDEV, qd.mkSession, nil); err != nil {
		return err
	}
	return ssrv.RunServer()
}
