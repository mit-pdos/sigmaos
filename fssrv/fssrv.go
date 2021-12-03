package fssrv

import (
	"log"
	"runtime/debug"

	"ulambda/fs"
	"ulambda/netsrv"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/watch"
)

type FsServer struct {
	addr  string
	root  fs.Dir
	mkps  protsrv.MakeProtServer
	stats *stats.Stats
	wt    *watch.WatchTable
	st    *session.SessionTable
	ct    *ConnTable
	srv   *netsrv.NetServer
	pclnt *procclnt.ProcClnt
	ch    chan bool
}

func MakeFsServer(root fs.Dir, addr string,
	mkps protsrv.MakeProtServer, pclnt *procclnt.ProcClnt,
	config repl.Config) *FsServer {
	fssrv := &FsServer{}
	fssrv.root = root
	fssrv.addr = addr
	fssrv.mkps = mkps
	fssrv.stats = stats.MkStats(fssrv.root)
	fssrv.wt = watch.MkWatchTable()
	fssrv.ct = MkConnTable()
	fssrv.st = session.MakeSessionTable()
	fssrv.srv = netsrv.MakeReplicatedNetServer(fssrv, addr, false, config)
	fssrv.pclnt = pclnt
	fssrv.ch = make(chan bool)
	return fssrv
}

func (fssrv *FsServer) Serve() {
	// Non-intial-named services wait on the pclnt infrastructure. Initial named waits on the channel.
	if fssrv.pclnt != nil {
		if err := fssrv.pclnt.Started(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("Error Started: %v", err)
		}
		if err := fssrv.pclnt.WaitEvict(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("Error WaitEvict: %v", err)
		}
	} else {
		<-fssrv.ch
	}
}

func (fssrv *FsServer) Done() {
	if fssrv.pclnt != nil {
		if err := fssrv.pclnt.Exited(proc.GetPid(), "EVICTED"); err != nil {
			debug.PrintStack()
			log.Printf("Error Exited: %v", err)
		}
	} else {
		fssrv.ch <- true
	}
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

func (fssrv *FsServer) AttachTree(uname string, aname string) (fs.Dir, fs.CtxI) {
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
