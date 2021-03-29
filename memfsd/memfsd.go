package memfsd

import (
	"net"

	db "ulambda/debug"
	"ulambda/memfs"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
)

type Fsd struct {
	root  *memfs.Inode
	srv   *npsrv.NpServer
	ch    chan bool
	addr  string
	mkctx func(string) npo.CtxI
	wt    *npo.WatchTable
}

func MakeFsd(addr string, mkctx func(string) npo.CtxI) *Fsd {
	fsd := &Fsd{}
	fsd.root = memfs.MkRootInode()
	fsd.addr = addr
	if mkctx == nil {
		fsd.mkctx = memfs.DefMkCtx
	} else {
		fsd.mkctx = mkctx
	}
	fsd.wt = npo.MkWatchTable()
	fsd.ch = make(chan bool)
	fsd.srv = npsrv.MakeNpServer(fsd, addr)
	return fsd
}

func (fsd *Fsd) Serve() {
	<-fsd.ch
	db.DLPrintf("NAMED", "Exit\n")
}

func (fsd *Fsd) Done() {
	db.DLPrintf("NAMED", "Done\n")
	fsd.ch <- true
}

func (fsd *Fsd) WatchTable() *npo.WatchTable {
	return fsd.wt
}

func (fsd *Fsd) Addr() string {
	return fsd.srv.MyAddr()
}

func (fsd *Fsd) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return fsd.root, fsd.mkctx(uname)
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(fsd, conn)
}

func (fsd *Fsd) MkNod(name string, d memfs.Data) error {
	obj, err := fsd.root.Create(fsd.mkctx(""), name, np.DMDEVICE, 0)
	if err != nil {
		return err
	}
	obj.(*memfs.Inode).Data = d
	return nil
}

func (fsd *Fsd) MkPipe(name string) (*memfs.Inode, error) {
	obj, err := fsd.root.Create(fsd.mkctx(""), name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj.(*memfs.Inode), nil
}
