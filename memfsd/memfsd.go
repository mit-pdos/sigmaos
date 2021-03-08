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
	root *memfs.Inode
	srv  *npsrv.NpServer
	ch   chan bool
	addr string
	ctx  *npo.Ctx
	r    npo.Resolver
}

func MakeFsd(uname, addr string, r npo.Resolver) *Fsd {
	fsd := &Fsd{}
	fsd.ctx = npo.MkCtx(uname, r)
	fsd.root = memfs.MkRootInode(fsd.ctx)
	fsd.addr = addr
	fsd.r = r
	fsd.srv = npsrv.MakeNpServer(fsd, addr)
	fsd.ch = make(chan bool)
	db.SetDebug(false)
	return fsd
}

func (fsd *Fsd) Serve() {
	<-fsd.ch
	db.DPrintf("Exit\n")
}

func (fsd *Fsd) Done() {
	db.DPrintf("Done\n")
	fsd.ch <- true
}

func (fsd *Fsd) Addr() string {
	return fsd.srv.MyAddr()
}

func (fsd *Fsd) Ctx() *npo.Ctx {
	return fsd.ctx
}

func (fsd *Fsd) Root() npo.NpObj {
	return fsd.root
}

func (fsd *Fsd) Resolver() npo.Resolver {
	return fsd.r
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(fsd, conn)
}

func (fsd *Fsd) MkNod(name string, d memfs.Data) error {
	obj, err := fsd.root.Create(fsd.ctx, name, np.DMDEVICE, 0)
	if err != nil {
		return err
	}
	obj.(*memfs.Inode).Data = d
	return nil
}

func (fsd *Fsd) MkPipe(name string) (*memfs.Inode, error) {
	obj, err := fsd.root.Create(fsd.ctx, name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj.(*memfs.Inode), nil
}
