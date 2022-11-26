package dbd

import (
	// db "sigmaos/debug"
	"sigmaos/filedev"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
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
	pds, err := protdevsrv.MakeProtDevSrv(np.DB, s)
	if err != nil {
		return err
	}
	qd := &queryDev{dbdaddr}
	if err := filedev.MkFileDev(pds.MemFs, QDEV, qd.mkSession); err != nil {
		return err
	}
	return pds.RunServer()
}
