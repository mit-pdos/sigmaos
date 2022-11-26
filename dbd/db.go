package dbd

import (
	// db "sigmaos/debug"
	"sigmaos/clonedev"
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
	pds, err := protdevsrv.MakeProtDevSrv(np.DB, s)
	if err != nil {
		return err
	}
	fd := mkFileDev(dbdaddr, pds.MemFs)
	if err := clonedev.MkCloneDev(pds.MemFs, CLONEFDEV, fd.mkSession, fd.detachSession); err != nil {
		return nil
	}
	return pds.RunServer()
}
