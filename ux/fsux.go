package fsux

import (
	"os"
	"sync"
	"syscall"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fslibsrv"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	// "sigmaos/seccomp"
)

var fsux *FsUx

type FsUx struct {
	*sesssrv.SessSrv
	*sigmaclnt.SigmaClnt
	mount string

	sync.Mutex
	ot *ObjTable
}

func RunFsUx(rootux string) {
	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}
	fsux = MakeReplicatedFsUx(rootux, ip+":0", proc.GetPid(), nil)
	fsux.Serve()
	fsux.Done()
}

func MakeReplicatedFsUx(rootux string, addr string, pid proc.Tpid, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux = &FsUx{}
	fsux.ot = MkObjTable()
	root, err := makeDir([]string{rootux})
	if err != nil {
		db.DFatalf("%v: makeDir %v\n", proc.GetName(), err)
	}
	srv, error := fslibsrv.MakeReplServer(root, addr, sp.UX, "ux", config)
	if error != nil {
		db.DFatalf("%v: MakeReplServer %v\n", proc.GetName(), error)
	}
	fsux.SessSrv = srv
	fsux.SigmaClnt = srv.SigmaClnt()
	return fsux
}

func ErrnoToNp(errno syscall.Errno, err error, name string) *serr.Err {
	switch errno {
	case syscall.ENOENT:
		return serr.MkErr(serr.TErrNotfound, name)
	case syscall.EEXIST:
		return serr.MkErr(serr.TErrExists, name)
	default:
		return serr.MkErrError(err)
	}
}

func UxTo9PError(err error, name string) *serr.Err {
	switch e := err.(type) {
	case *os.LinkError:
		return ErrnoToNp(e.Err.(syscall.Errno), err, name)
	case *os.PathError:
		return ErrnoToNp(e.Err.(syscall.Errno), err, name)
	case syscall.Errno:
		return ErrnoToNp(e, err, name)
	default:
		return serr.MkErrError(err)
	}
}
