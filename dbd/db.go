package dbd

import (
	// db "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/sessdevsrv"
	sp "sigmaos/sigmap"
)

//
// mysql client exporting a database server through the file system
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
	pds, err := protdevsrv.MakeProtDevSrv(sp.DB, s)
	if err != nil {
		return err
	}
	qd := &queryDev{dbdaddr}
	if err := sessdevsrv.MkSessDev(pds.MemFs, QDEV, qd.mkSession, nil); err != nil {
		return err
	}
	return pds.RunServer()
}
