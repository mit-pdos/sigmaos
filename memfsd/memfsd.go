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
	root  *memfs.Dir
	srv   *npsrv.NpServer
	ch    chan bool
	addr  string
	mkctx func(string) npo.CtxI
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

func (fsd *Fsd) Addr() string {
	return fsd.srv.MyAddr()
}

func (fsd *Fsd) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return fsd.root, fsd.mkctx(uname)
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(fsd, conn)
}

func (fsd *Fsd) MkNod(name string, d memfs.Dev) error {
	_, err := fsd.root.CreateDev(fsd.mkctx(""), name, d, np.DMDEVICE, 0)
	return err
}

func (fsd *Fsd) MkPipe(name string) (npo.NpObj, error) {
	obj, err := fsd.root.Create(fsd.mkctx(""), name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
