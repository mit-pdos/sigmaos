package fssrv

import (
	"ulambda/fs"
	"ulambda/netsrv"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/watch"
)

type Fs interface {
	Done()
}

type FsServer struct {
	fs    Fs
	addr  string
	root  fs.FsObj
	mkps  protsrv.MakeProtServer
	stats *stats.Stats
	wt    *watch.WatchTable
	st    *session.SessionTable
	ct    *ConnTable
	srv   *netsrv.NetServer
}

func MakeFsServer(fs Fs, root fs.FsObj, addr string,
	mkps protsrv.MakeProtServer,
	config repl.Config) *FsServer {
	fssrv := &FsServer{}
	fssrv.fs = fs
	fssrv.root = root
	fssrv.addr = addr
	fssrv.mkps = mkps
	fssrv.stats = stats.MkStats()
	fssrv.wt = watch.MkWatchTable()
	fssrv.ct = MkConnTable()
	fssrv.st = session.MakeSessionTable()
	fssrv.srv = netsrv.MakeReplicatedNetServer(fssrv, addr, false, config)
	return fssrv
}

func (fssrv *FsServer) MyAddr() string {
	return fssrv.srv.MyAddr()
}

func (fssrv *FsServer) GetStats() *stats.Stats {
	return fssrv.stats
}

func (fssrv *FsServer) GetWatchTable() *watch.WatchTable {
	return fssrv.wt
}

func (fssrv *FsServer) SessionTable() *session.SessionTable {
	return fssrv.st
}

func (fssrv *FsServer) GetConnTable() *ConnTable {
	return fssrv.ct
}

func (fssrv *FsServer) Done() {
	fssrv.fs.Done()
}

func (fssrv *FsServer) RootAttach(uname string) (fs.FsObj, fs.CtxI) {
	return fssrv.root, MkCtx(uname)
}

func (fssrv *FsServer) Connect() protsrv.Protsrv {
	psrv := fssrv.mkps.MakeProtServer(fssrv)
	fssrv.ct.Add(psrv)
	return psrv
}

type Ctx struct {
	uname string
}

func MkCtx(uname string) *Ctx {
	return &Ctx{uname}
}

func (ctx *Ctx) Uname() string {
	return ctx.uname
}
