package dbd

import (
	"log"
	"runtime/debug"

	"ulambda/dir"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procclnt"
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
	mfs, err := fslibsrv.StartMemFs(named.DB, "dbd")
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	pc := procclnt.MakeProcClnt(mfs.FsLib)
	err = dir.MkNod(fssrv.MkCtx(""), mfs.Root(), "clone", makeClone("", mfs.Root()))
	if err != nil {
		log.Fatalf("MakeNod clone failed %v\n", err)
	}
	if err := pc.Started(proc.GetPid()); err != nil {
		debug.PrintStack()
		log.Fatalf("Error Started: %v", err)
	}
	if err := pc.WaitEvict(proc.GetPid()); err != nil {
		debug.PrintStack()
		log.Fatalf("Error WaitEvict: %v", err)
	}
	if err := pc.Exited(proc.GetPid(), "EVICTED"); err != nil {
		debug.PrintStack()
		log.Fatalf("Error Exited: %v", err)
	}

	//	mfs.Wait()
}
