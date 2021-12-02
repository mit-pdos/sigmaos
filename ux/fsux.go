package fsux

import (
	"log"
	"runtime/debug"
	"sync"

	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
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
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", named.UX, err)
	}
	fsux := MakeReplicatedFsUx(mount, ip+":0", proc.GetPid(), nil)
	pc := procclnt.MakeProcClnt(fsux.FsLib)
	if err := pc.Started(proc.GetPid()); err != nil {
		debug.PrintStack()
		log.Fatalf("Error Started: %v", err)
	}
	if err := pc.WaitEvict(proc.GetPid()); err != nil {
		debug.PrintStack()
		log.Fatalf("Error WaitEvict: %v", err)
	}
	if err := pc.Exited(proc.GetPid(), "EVICTED"); err != nil {
		debug.PrintStack()
		log.Fatalf("Error Exited: %v", err)
	}
}

func MakeReplicatedFsUx(mount string, addr string, pid string, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux := &FsUx{}
	root := makeDir([]string{mount}, np.DMDIR, nil)
	srv, fsl, err := fslibsrv.MakeReplServer(root, addr, named.UX, "ux", config)
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}
	fsux.FsServer = srv
	fsux.FsLib = fsl
	return fsux
}
