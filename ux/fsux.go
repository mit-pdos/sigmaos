package fsux

import (
	"log"
	"path"
	"sync"

	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/repl"
	usync "ulambda/sync"
	// "ulambda/seccomp"
)

type FsUx struct {
	*fssrv.FsServer
	mu    sync.Mutex
	root  fs.Dir
	mount string
}

func MakeFsUx(mount string, pid string) *FsUx {
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", named.UX, err)
	}
	return MakeReplicatedFsUx(mount, ip+":0", pid, nil)
}

func MakeReplicatedFsUx(mount string, addr string, pid string, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux := &FsUx{}
	fsux.root = makeDir([]string{mount}, np.DMDIR, nil)
	srv, fsl, err := fslibsrv.MakeReplSrvFsLib(fsux.root, addr, named.UX, "ux", config)
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}
	fsux.FsServer = srv
	if config == nil {
		fsuxStartCond := usync.MakeCond(fsl, path.Join(named.BOOT, pid), nil)
		fsuxStartCond.Destroy()
	}
	return fsux
}
