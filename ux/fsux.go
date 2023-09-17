package fsux

import (
	"sync"

	"sigmaos/proc"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	// "sigmaos/seccomp"
)

var fsux *FsUx

type FsUx struct {
	*sigmaclnt.SigmaClnt
	*sigmasrv.SigmaSrv
	mount string

	sync.Mutex
	ot *ObjTable
}

func RunFsUx(rootux string) {
	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux := newUx(rootux)
	root, sr := newDir([]string{rootux})
	if sr != nil {
		db.DFatalf("newDir %v\n", sr)
	}
	pcfg := proc.GetProcEnv()
	srv, err := sigmasrv.NewSigmaSrvRoot(root, ip+":0", sp.UX, pcfg)
	if err != nil {
		db.DFatalf("BootSrvAndPost %v\n", err)
	}
	fsux.SigmaSrv = srv
	fsux.RunServer()
}

func newUx(rootux string) *FsUx {
	fsux = &FsUx{}
	fsux.ot = MkObjTable()
	return fsux
}
