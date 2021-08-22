package fssrv

import (
	"ulambda/fs"
	"ulambda/npapi"
	"ulambda/npsrv"
	"ulambda/session"
	"ulambda/stats"
)

type Fs interface {
	Done()
}

type FsServer struct {
	fs    Fs
	addr  string
	root  fs.NpObj
	npcm  npapi.NpConnMaker
	stats *stats.Stats
	wt    *WatchTable
	st    *session.SessionTable
	ct    *ConnTable
	srv   *npsrv.NpServer
}

func MakeFsServer(fs Fs, root fs.NpObj, addr string,
	npcm npapi.NpConnMaker,
	replicated bool,
	relayAddr string, config *npsrv.NpServerReplConfig) *FsServer {
	fssrv := &FsServer{}
	fssrv.root = root
	fssrv.addr = addr
	fssrv.npcm = npcm
	fssrv.stats = stats.MkStats()
	fssrv.wt = MkWatchTable()
	fssrv.ct = MkConnTable()
	fssrv.srv = npsrv.MakeReplicatedNpServer(fssrv, addr, false, replicated, relayAddr, config)
	return fssrv
}

func (fssrv *FsServer) MyAddr() string {
	return fssrv.srv.MyAddr()
}

func (fssrv *FsServer) GetStats() *stats.Stats {
	return fssrv.stats
}

func (fssrv *FsServer) GetWatchTable() *WatchTable {
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

func (fssrv *FsServer) RootAttach(uname string) (fs.NpObj, fs.CtxI) {
	return fssrv.root, MkCtx(uname)
}

func (fssrv *FsServer) Connect() npapi.NpAPI {
	conn := fssrv.npcm.MakeNpConn(fssrv)
	fssrv.ct.Add(conn)
	return conn
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
