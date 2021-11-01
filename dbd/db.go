package dbd

import (
	"log"
	"path"

	"ulambda/dir"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/named"
	usync "ulambda/sync"
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

func RunDbd(pid string) {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	mfs, err := fslibsrv.StartMemFs(named.DB, "dbd")
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	dbdStartCond := usync.MakeCond(mfs.FsLib, path.Join(named.BOOT, pid), nil, true)
	dbdStartCond.Destroy()
	err = dir.MkNod(fssrv.MkCtx(""), mfs.Root(), "clone", makeClone("", mfs.Root()))
	if err != nil {
		log.Fatalf("MakeNod clone failed %v\n", err)
	}
	mfs.Wait()
}
