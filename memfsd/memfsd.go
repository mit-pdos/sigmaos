package memfsd

import (
	"log"
	"net"
	"sync"

	db "ulambda/debug"
	"ulambda/memfs"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
	"ulambda/stats"
)

const MEMFS = "name/memfsd"

type Ctx struct {
	uname string
}

func MkCtx(uname string) *Ctx {
	return &Ctx{uname}
}

func (ctx *Ctx) Uname() string {
	return ctx.uname
}

type Fsd struct {
	mu    sync.Mutex
	root  *memfs.Dir
	srv   *npsrv.NpServer
	ch    chan bool
	addr  string
	wt    *npo.WatchTable
	ct    *npo.ConnTable
	st    *npo.SessionTable
	stats *stats.Stats
}

func MakeFsd(addr string) *Fsd {
	return MakeReplicatedFsd(addr, false, "", nil)
}

func MakeReplicatedFsd(addr string, replicated bool, relayAddr string, config *npsrv.NpServerReplConfig) *Fsd {
	fsd := &Fsd{}
	fsd.root = memfs.MkRootInode()
	fsd.addr = addr
	fsd.wt = npo.MkWatchTable()
	fsd.ct = npo.MkConnTable()
	fsd.st = npo.MakeSessionTable()
	fsd.stats = stats.MkStats()
	fsd.ch = make(chan bool)
	fsd.srv = npsrv.MakeReplicatedNpServer(fsd, addr, false, replicated, relayAddr, config)
	if err := fsd.MkNod("statsd", fsd.stats); err != nil {
		log.Fatalf("Mknod failed %v\n", err)
	}
	return fsd
}

func (fsd *Fsd) GetSrv() *npsrv.NpServer {
	return fsd.srv
}

func (fsd *Fsd) Serve() {
	<-fsd.ch
	db.DLPrintf("MEMFSD", "Exit\n")
}

func (fsd *Fsd) Done() {
	db.DLPrintf("MEMFSD", "Done\n")
	fsd.ch <- true
}

func (fsd *Fsd) WatchTable() *npo.WatchTable {
	return fsd.wt
}

func (fsd *Fsd) ConnTable() *npo.ConnTable {
	return fsd.ct
}

func (fsd *Fsd) SessionTable() *npo.SessionTable {
	return fsd.st
}

func (fsd *Fsd) RegisterSession(sess np.Tsession) {
	fsd.st.RegisterSession(sess)
}

func (fsd *Fsd) Stats() *stats.Stats {
	return fsd.stats
}

func (fsd *Fsd) Addr() string {
	return fsd.srv.MyAddr()
}

func (fsd *Fsd) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return fsd.root, MkCtx(uname)
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(fsd, conn)
}

func (fsd *Fsd) MkNod(name string, d memfs.Dev) error {
	_, err := fsd.root.CreateDev(MkCtx(""), name, d, np.DMDEVICE, 0)
	return err
}

func (fsd *Fsd) MkPipe(name string) (npo.NpObj, error) {
	obj, err := fsd.root.Create(MkCtx(""), name, np.DMNAMEDPIPE, 0)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
