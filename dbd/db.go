package dbd

import (
	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fslibsrv"
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
	mfs, _, _, error := fslibsrv.MakeMemFs(np.DB, "dbd")
	if error != nil {
		db.DFatalf("FATAL StartMemFs %v\n", error)
	}
	err := dir.MkNod(ctx.MkCtx("", 0, nil), mfs.Root(), "clone", makeClone(nil, mfs.Root()))
	if err != nil {
		db.DFatalf("FATAL MakeNod clone failed %v\n", err)
	}
	mfs.Serve()
	mfs.Done()
}
