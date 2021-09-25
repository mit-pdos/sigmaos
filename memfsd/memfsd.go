package memfsd

import (
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fsimpl"
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
	ch    chan bool
}

func MakeFsd(addr string) *Fsd {
	return MakeReplicatedFsd(addr, nil)
}

func MakeReplicatedFsd(addr string, config repl.Config) *Fsd {
	fsd := &Fsd{}
	fsd.root = fsimpl.MkRootDir(memfs.MakeInode)
	fsd.fssrv = fssrv.MakeFsServer(fsd, fsd.root.(fs.FsObj),
		addr, fsobjsrv.MakeProtServer(), config)
	fsd.ch = make(chan bool)
	err := memfs.MkNod(fssrv.MkCtx(""), fsd.root, "statsd", fsd.fssrv.GetStats())
	if err != nil {
		log.Fatalf("MakeNod failed %v\n", err)
	}
	return fsd
}

func (fsd *Fsd) Serve() {
	<-fsd.ch
	db.DLPrintf("MEMFSD", "Exit\n")
}

func (fsd *Fsd) Done() {
	db.DLPrintf("MEMFSD", "Done\n")
	fsd.ch <- true
}

func (fsd *Fsd) GetSrv() *fssrv.FsServer {
	return fsd.fssrv
}

func (fsd *Fsd) Addr() string {
	return fsd.fssrv.MyAddr()
}

func (fsd *Fsd) GetRoot() fs.Dir {
	return fsd.root
}

func (fsd *Fsd) MkPipe(name string) (fs.FsObj, error) {
	obj, err := fsd.root.Create(fssrv.MkCtx(""), name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj, nil

}
