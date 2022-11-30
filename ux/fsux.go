package fsux

import (
	"os"
	"sync"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	"sigmaos/fslibsrv"
	np "sigmaos/sigmap"
    "sigmaos/fcall"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/sesssrv"
	// "sigmaos/seccomp"
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

func ErrnoToNp(errno syscall.Errno, err error, name string) *fcall.Err {
	switch errno {
	case syscall.ENOENT:
		return fcall.MkErr(fcall.TErrNotfound, name)
	case syscall.EEXIST:
		return fcall.MkErr(fcall.TErrExists, name)
	default:
		return fcall.MkErrError(err)
	}
}

func UxTo9PError(err error, name string) *fcall.Err {
	switch e := err.(type) {
	case *os.LinkError:
		return ErrnoToNp(e.Err.(syscall.Errno), err, name)
	case *os.PathError:
		return ErrnoToNp(e.Err.(syscall.Errno), err, name)
	case syscall.Errno:
		return ErrnoToNp(e, err, name)
	default:
		return fcall.MkErrError(err)
	}
}
