package fsux

import (
	"sync"

	db "ulambda/debug"
	"ulambda/fidclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	np "ulambda/ninep"
	"ulambda/pathmap"
	"ulambda/proc"
	"ulambda/repl"
	"ulambda/sesssrv"
	// "ulambda/seccomp"
)

var paths *pathmap.PathMap

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
	paths = pathmap.MkPathMap()
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
