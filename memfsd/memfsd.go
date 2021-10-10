package memfsd

import (
	"log"
	"sync"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/memfs"
	"ulambda/repl"
)

const MEMFS = "name/memfsd"

type Fsd struct {
	*fssrv.FsServer
	mu   sync.Mutex
	root fs.Dir
}

func MakeFsd(addr string) *Fsd {
	return MakeReplicatedFsd(addr, nil)
}

func MakeReplicatedFsd(addr string, config repl.Config) *Fsd {
	fsd := &Fsd{}
	fsd.root = dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	fsd.FsServer = fssrv.MakeFsServer(fsd.root,
		addr, fsobjsrv.MakeProtServer(), config)
	err := dir.MkNod(fssrv.MkCtx(""), fsd.root, "statsd", fsd.FsServer.GetStats())
	if err != nil {
		log.Fatalf("MakeNod failed %v\n", err)
	}
	return fsd
}
