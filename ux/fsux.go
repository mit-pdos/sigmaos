package fsux

import (
	"os"
	"sync"
	"syscall"

	db "ulambda/debug"
	"ulambda/fidclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/repl"
	"ulambda/sesssrv"
	// "ulambda/seccomp"
)

var fsux *FsUx

type FsUx struct {
	*sesssrv.SessSrv
	*fslib.FsLib
	mount string

	sync.Mutex
	ot *ObjTable
}

func RunFsUx(mount string) {
	ip, err := fidclnt.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", np.UX, err)
	}
	fsux = MakeReplicatedFsUx(mount, ip+":0", proc.GetPid(), nil)
	fsux.Serve()
	fsux.Done()
}

func MakeReplicatedFsUx(mount string, addr string, pid proc.Tpid, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux = &FsUx{}
	fsux.ot = MkObjTable()
	root, err := makeDir([]string{mount})
	if err != nil {
		db.DFatalf("%v: makeDir %v\n", proc.GetName(), err)
	}
	srv, fsl, _, error := fslibsrv.MakeReplServer(root, addr, np.UX, "ux", config)
	if error != nil {
		db.DFatalf("%v: MakeReplServer %v\n", proc.GetName(), error)
	}
	fsux.SessSrv = srv
	fsux.FsLib = fsl
	return fsux
}

func ErrnoToNp(errno syscall.Errno, err error) *np.Err {
	switch errno {
	case syscall.ENOENT:
		return np.MkErr(np.TErrNotfound, err)
	case syscall.EEXIST:
		return np.MkErr(np.TErrExists, err)
	default:
		return np.MkErrError(err)
	}
}

func UxTo9PError(err error) *np.Err {
	switch e := err.(type) {
	case *os.LinkError:
		return ErrnoToNp(e.Err.(syscall.Errno), err)
	case *os.PathError:
		return ErrnoToNp(e.Err.(syscall.Errno), err)
	case syscall.Errno:
		return ErrnoToNp(e, err)
	default:
		return np.MkErrError(err)
	}
}
