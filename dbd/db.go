package dbd

import (
	"log"
	"path"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/memfs"
	"ulambda/named"
	np "ulambda/ninep"
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

type Database struct {
	fssrv  *fssrv.FsServer
	ch     chan bool
	root   fs.Dir
	nextId np.Tpath
}

func MakeDbd(pid string) (*Database, error) {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	dbd := &Database{}
	dbd.ch = make(chan bool)
	dbd.root = dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	srv, fsl, err := fslibsrv.MakeSrvFsLib(dbd, dbd.root, named.DB, "dbd")
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}
	dbd.fssrv = srv
	dbdStartCond := usync.MakeCond(fsl, path.Join(named.BOOT, pid), nil)
	dbdStartCond.Destroy()
	err = dir.MkNod(fssrv.MkCtx(""), dbd.root, "clone", makeClone("", dbd.root))
	if err != nil {
		log.Fatalf("MakeNod clone failed %v\n", err)
	}
	return dbd, nil
}

func (dbd *Database) Serve() {
	<-dbd.ch
}

func (dbd *Database) Done() {
	dbd.ch <- true
}
