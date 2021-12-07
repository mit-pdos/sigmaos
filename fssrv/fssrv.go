package fssrv

import (
	"log"
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/netsrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/watch"
)

type FsServer struct {
	protsrv.Protsrv
	addr  string
	root  fs.Dir
	mkps  protsrv.MakeProtServer
	stats *stats.Stats
	wt    *watch.WatchTable
	st    *session.SessionTable
	ct    *ConnTable
	srv   *netsrv.NetServer
	pclnt *procclnt.ProcClnt
	done  bool
	ch    chan bool
	fsl   *fslib.FsLib
}

func MakeFsServer(root fs.Dir, addr string, fsl *fslib.FsLib,
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
	fssrv.fsl = fsl
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
		if !fssrv.done {
			fssrv.done = true
			fssrv.ch <- true
		}
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
	fssrv.Protsrv = fssrv.mkps.MakeProtServer(fssrv)
	fssrv.ct.Add(fssrv.Protsrv)
	return fssrv
}

func (fssrv *FsServer) Register(sess np.Tsession, args np.Tregister, rets *np.Ropen) *np.Rerror {
	log.Printf("%v: reg %v %v\n", db.GetName(), sess, args)
	if err := fssrv.st.Lock(sess, args.Wnames, args.Qid); err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (fssrv *FsServer) Deregister(sess np.Tsession, args np.Tderegister, rets *np.Ropen) *np.Rerror {
	log.Printf("%v: dereg %v %v\n", db.GetName(), sess, args)
	if err := fssrv.st.Unlock(sess, args.Wnames); err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (fssrv *FsServer) checkLock(sess np.Tsession) error {
	fn, err := fssrv.st.LockName(sess)
	if err != nil {
		return err
	}
	if fn == nil { // no lock on this session
		return nil
	}
	st, err := fssrv.fsl.Stat(np.Join(fn))
	if err != nil {
		return err
	}
	return fssrv.st.CheckLock(sess, fn, st.Qid)
}

func (fssrv *FsServer) Write(sess np.Tsession, args np.Twrite, rets *np.Rwrite) *np.Rerror {
	err := fssrv.checkLock(sess)
	if err != nil {
		log.Printf("checkDlock %v\n", err)
		return &np.Rerror{err.Error()}
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
