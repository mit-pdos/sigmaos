package dbd

import (
	// db "sigmaos/debug"
	"sigmaos/sigmasrv"
	"sigmaos/sessdevsrv"
	sp "sigmaos/sigmap"
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
	ssrv, err := sigmasrv.MakeSigmaSrv(sp.DB, s, sp.DB)
	if err != nil {
		return err
	}
	qd := &queryDev{dbdaddr}
	if err := sessdevsrv.MkSessDev(ssrv.MemFs, QDEV, qd.mkSession, nil); err != nil {
		return err
	}
	return ssrv.RunServer()
}
