package fssrv

import (
	"fmt"
	"log"

	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/netsrv"
	np "ulambda/ninep"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/watch"
)

type Dlock struct {
	fn []string
	v  np.TQversion
}

func makeDlock(dlock []string, v np.TQversion) *Dlock {
	return &Dlock{dlock, v}
}

type FsServer struct {
	protsrv.Protsrv
	addr  string
	root  fs.Dir
	dlock *Dlock
	mkps  protsrv.MakeProtServer
	stats *stats.Stats
	wt    *watch.WatchTable
	st    *session.SessionTable
	ct    *ConnTable
	srv   *netsrv.NetServer
	ch    chan bool
	fsl   *fslib.FsLib
}

func MakeFsServer(root fs.Dir, addr string, fsl *fslib.FsLib, mkps protsrv.MakeProtServer, config repl.Config) *FsServer {
	fssrv := &FsServer{}
	fssrv.root = root
	fssrv.addr = addr
	fssrv.mkps = mkps
	fssrv.stats = stats.MkStats(fssrv.root)
	fssrv.wt = watch.MkWatchTable()
	fssrv.ct = MkConnTable()
	fssrv.st = session.MakeSessionTable()
	fssrv.srv = netsrv.MakeReplicatedNetServer(fssrv, addr, false, config)
	fssrv.ch = make(chan bool)
	fssrv.fsl = fsl
	return fssrv
}

func (fsd *FsServer) Serve() {
	<-fsd.ch
}

func (fsd *FsServer) Done() {
	fsd.ch <- true
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
	fssrv.Protsrv = fssrv.mkps.MakeProtServer(fssrv)
	fssrv.ct.Add(fssrv.Protsrv)
	return fssrv
}

func (fssrv *FsServer) checkDlock() error {
	st, err := fssrv.fsl.Stat(np.Join(fssrv.dlock.fn))
	if err != nil {
		return err
	}
	if st.Qid.Version != fssrv.dlock.v {
		return fmt.Errorf("dlock %v is stale\n", fssrv.dlock.fn)
	}
	return nil
}

func (fssrv *FsServer) Register(sess np.Tsession, args np.Tregister, rets *np.Ropen) *np.Rerror {
	// log.Printf("%v: reg %v\n", db.GetName(), args)
	fssrv.dlock = makeDlock(args.Wnames, args.Version)
	return nil
}

func (fssrv *FsServer) Deregister(sess np.Tsession, args np.Tderegister, rets *np.Ropen) *np.Rerror {
	// log.Printf("%v: dereg %v\n", db.GetName(), args)
	if fssrv.dlock == nil {
		return &np.Rerror{fmt.Sprintf("lock %v is stale")}
	}
	fssrv.dlock = nil
	return nil
}

func (fssrv *FsServer) Write(sess np.Tsession, args np.Twrite, rets *np.Rwrite) *np.Rerror {
	if fssrv.dlock != nil {
		err := fssrv.checkDlock()
		if err != nil {
			log.Printf("checkDlock %v\n", err)
			return &np.Rerror{err.Error()}
		}
	}
	return fssrv.Protsrv.Write(sess, args, rets)
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
