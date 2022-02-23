package fsux

import (
	"log"
	"sync"

	"ulambda/fidclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/repl"
	// "ulambda/seccomp"
)

type FsUx struct {
	*fssrv.FsServer
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

func MakeReplicatedFsUx(mount string, addr string, pid string, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux := &FsUx{}
	root := makeDir([]string{mount}, np.DMDIR, nil)
	srv, fsl, _, err := fslibsrv.MakeReplServer(root, addr, np.UX, "ux", config)
	if err != nil {
		log.Fatalf("%v: MakeReplServer %v\n", proc.GetProgram(), err)
	}
	fsux.FsServer = srv
	fsux.FsLib = fsl
	return fsux
}
