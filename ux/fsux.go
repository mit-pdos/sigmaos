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

type FsUx struct {
	*sesssrv.SessSrv
	*fslib.FsLib
	mu    sync.Mutex
	mount string
}

func RunFsUx(mount string) {
	ip, err := fidclnt.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", np.UX, err)
	}
	fsux := MakeReplicatedFsUx(mount, ip+":0", proc.GetPid(), nil)
	fsux.Serve()
	fsux.Done()
}

func MakeReplicatedFsUx(mount string, addr string, pid proc.Tpid, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux := &FsUx{}
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
	if e, ok := err.(*os.LinkError); ok {
		return ErrnoToNp(e.Err.(syscall.Errno), err)
	}
	if e, ok := err.(*os.PathError); ok {
		return ErrnoToNp(e.Err.(syscall.Errno), err)
	}
	return np.MkErrError(err)
}
