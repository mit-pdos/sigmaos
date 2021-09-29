package dbd

import (
	"log"
	"path"

	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslib"
	fos "ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/memfs"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/repl"
	usync "ulambda/sync"
)

//
// mysql client exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

const (
	DBD = "name/db/mydb/"
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

func MakeDbd(addr, pid string) (*Database, error) {
	return MakeReplicatedDbd(addr, pid, false, nil)
}

func MakeReplicatedDbd(addr string, pid string, replicated bool, config repl.Config) (*Database, error) {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	dbd := &Database{}
	dbd.ch = make(chan bool)
	dbd.root = dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	db.Name("dbd")
	dbd.fssrv = fssrv.MakeFsServer(dbd, dbd.root, addr, fos.MakeProtServer(), config)
	fsl := fslib.MakeFsLib("dbd")
	fsl.Mkdir(named.DB, 0777)
	err := fsl.PostServiceUnion(dbd.fssrv.MyAddr(), named.DB, "mydb")
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", dbd.fssrv.MyAddr(), err)
	}

	if !replicated {
		dbdStartCond := usync.MakeCond(fsl, path.Join(named.BOOT, pid), nil)
		dbdStartCond.Destroy()
	}
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
