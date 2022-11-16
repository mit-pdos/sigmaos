package dbd

import (
	// db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

type Book struct {
	Author string
	Price  string
	Title  string
}

//
// mysql client exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

func RunDbd() error {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	s, err := mkServer()
	if err != nil {
		return err
	}
	pds := protdevsrv.MakeProtDevSrv(np.DB, s)
	return pds.RunServer()
}
