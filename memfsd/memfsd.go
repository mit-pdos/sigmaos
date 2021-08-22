package memfsd

import (
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fssrv"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/npobjsrv"
	"ulambda/npsrv"
	"ulambda/seccomp"
)

const MEMFS = "name/memfsd"

type Fsd struct {
	mu    sync.Mutex
	fssrv *fssrv.FsServer
	root  *memfs.Dir
	ch    chan bool
}

func MakeFsd(addr string) *Fsd {
	return MakeReplicatedFsd(addr, false, "", nil)
}

func MakeReplicatedFsd(addr string, replicated bool, relayAddr string, config *npsrv.NpServerReplConfig) *Fsd {
	seccomp.LoadFilter()
	fsd := &Fsd{}
	fsd.root = memfs.MkRootInode()
	fsd.fssrv = fssrv.MakeFsServer(fsd, fsd.root,
		addr, npobjsrv.MakeConnMaker(), replicated, relayAddr, config)
	fsd.ch = make(chan bool)
	if err := fsd.MkNod("statsd", fsd.fssrv.GetStats()); err != nil {
		log.Fatalf("Mknod failed %v\n", err)
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

func (fsd *Fsd) MkNod(name string, d memfs.Dev) error {
	_, err := fsd.root.CreateDev(fssrv.MkCtx(""), name, d, np.DMDEVICE, 0)
	return err
}

func (fsd *Fsd) MkPipe(name string) (fs.NpObj, error) {
	obj, err := fsd.root.Create(fssrv.MkCtx(""), name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
