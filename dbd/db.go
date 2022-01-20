package dbd

import (
	"log"

	"ulambda/dir"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	np "ulambda/ninep"
)

//
// mysql client exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

const (
	DBD = "name/db/~ip/"
)

type Book struct {
	Author string
	Price  string
	Title  string
}

func RunDbd() {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	mfs, _, err := fslibsrv.MakeMemFs(np.DB, "dbd")
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	err = dir.MkNod(fssrv.MkCtx("", 0, nil), mfs.Root(), "clone", makeClone(nil, mfs.Root()))
	if err != nil {
		log.Fatalf("MakeNod clone failed %v\n", err)
	}
	mfs.Serve()
	mfs.Done()
}
