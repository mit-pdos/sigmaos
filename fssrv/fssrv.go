package fssrv

import (
	"log"
	"runtime/debug"

	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/netsrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/stats"
	"ulambda/watch"
)

//
// There is one FsServer per memfsd. The FsServer has one ProtSrv per
// 9p channel (i.e., TCP connection); each channel has one or more
// sessions (one per client fslib on the same client machine).
//

type FsServer struct {
	addr  string
	root  fs.Dir
	mkps  protsrv.MakeProtServer
	stats *stats.Stats
	st    *sessionTable
	wt    *watch.WatchTable
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
	fssrv.st = makeSessionTable(fssrv)
	fssrv.wt = watch.MkWatchTable()
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

func (fssrv *FsServer) AttachTree(uname string, aname string) (fs.Dir, fs.CtxI) {
	return fssrv.root, MkCtx(uname)
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

// XXX fix me
func (fssrv *FsServer) Write(sess np.Tsession, args np.Twrite, rets *np.Rwrite) *np.Rerror {
	err := fssrv.checkLock(sess)
	if err != nil {
		log.Printf("checkDlock %v\n", err)
		return &np.Rerror{err.Error()}
	}
	_, r := fssrv.Dispatch(sess, args)
	return r
}

func (fssrv *FsServer) Detach(sess np.Tsession) {
	fssrv.st.detach()
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
