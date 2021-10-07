package memfsd

import (
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/repl"
)

const MEMFS = "name/memfsd"

type Fsd struct {
	mu    sync.Mutex
	fssrv *fssrv.FsServer
	root  fs.Dir
}

func MakeFsd(addr string) *Fsd {
	return MakeReplicatedFsd(addr, nil)
}

func MakeReplicatedFsd(addr string, config repl.Config) *Fsd {
	fsd := &Fsd{}
	fsd.root = dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	fsd.fssrv = fssrv.MakeFsServer(fsd.root,
		addr, fsobjsrv.MakeProtServer(), config)
	err := dir.MkNod(fssrv.MkCtx(""), fsd.root, "statsd", fsd.fssrv.GetStats())
	if err != nil {
		log.Fatalf("MakeNod failed %v\n", err)
	}
	return fsd
}

func (fsd *Fsd) Serve() {
	fsd.fssrv.Serve()
	db.DLPrintf("MEMFSD", "Exit\n")
}

func (fsd *Fsd) GetSrv() *fssrv.FsServer {
	return fsd.fssrv
}

func (fsd *Fsd) Addr() string {
	return fsd.fssrv.MyAddr()
}

func (fsd *Fsd) MkPipe(name string) (fs.FsObj, error) {
	obj, err := fsd.root.Create(fssrv.MkCtx(""), name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj, nil

}
