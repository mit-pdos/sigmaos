package fsux

import (
	"log"
	"sync"

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
		log.Fatalf("LocalIP %v %v\n", np.UX, err)
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
		log.Fatalf("%v: makeDir %v\n", proc.GetName(), err)
	}
	srv, fsl, _, error := fslibsrv.MakeReplServer(root, addr, np.UX, "ux", config)
	if error != nil {
		log.Fatalf("%v: MakeReplServer %v\n", proc.GetName(), error)
	}
	fsux.SessSrv = srv
	fsux.FsLib = fsl
	return fsux
}
