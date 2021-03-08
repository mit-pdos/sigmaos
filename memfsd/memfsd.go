package memfsd

import (
	"net"
	"sync"

	db "ulambda/debug"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/npobjsrv"
	"ulambda/npsrv"
)

// XXX maybe overload func (npc *Npconn) Open ..
type Walker interface {
	Walk(string, []string) error
}

type NpConn struct {
	mu    sync.Mutex // for Fids
	conn  net.Conn
	uname string
	walk  Walker
}

type Fsd struct {
	root *memfs.Inode
	srv  *npsrv.NpServer
	walk Walker
	ch   chan bool
	addr string
}

func MakeFsd(addr string, w Walker) *Fsd {
	fsd := &Fsd{}
	fsd.root = memfs.RootInode()
	fsd.walk = w
	fsd.addr = addr
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

func (fsd *Fsd) Root() npobjsrv.NpObj {
	return fsd.root
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	return npobjsrv.MakeNpConn(fsd, conn)
}

func (fsd *Fsd) MkNod(name string, d memfs.Data) error {
	obj, err := fsd.root.Create(name, np.DMDEVICE, 0)
	if err != nil {
		return err
	}
	obj.(*memfs.Inode).Data = d
	return nil
}

func (fsd *Fsd) MkPipe(uname string, name string) (*memfs.Inode, error) {
	obj, err := fsd.root.Create(name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj.(*memfs.Inode), nil
}
