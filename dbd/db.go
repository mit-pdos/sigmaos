package dbd

import (
	// db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

//
// mysql client exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

func RunDbd(dbdaddr string) error {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	s, err := mkServer(dbdaddr)
	if err != nil {
		return err
	}
	pds := protdevsrv.MakeProtDevSrv(np.DB, s)
	pds.QueueLen()
	return pds.RunServer()
}
