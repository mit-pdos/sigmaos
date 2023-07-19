package fsux

import (
	"sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fslibsrv"
	"sigmaos/proc"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	// "sigmaos/seccomp"
)

var fsux *FsUx

type FsUx struct {
	*sigmaclnt.SigmaClnt
	*sesssrv.SessSrv
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
	root, sr := makeDir([]string{rootux})
	if sr != nil {
		db.DFatalf("%v: makeDir %v\n", proc.GetName(), sr)
	}
	srv, error := fslibsrv.BootSrvAndPost(root, ip+":0", sp.UX, sp.UXREL)
	if error != nil {
		db.DFatalf("%v: MakeReplServer %v\n", proc.GetName(), error)
	}
	fsux.SessSrv = srv
	fsux.Serve()
	fsux.Done()
	fsux.Exited(proc.MakeStatus(proc.StatusEvicted))
}

func newUx(rootux string) *FsUx {
	fsux = &FsUx{}
	fsux.ot = MkObjTable()
	return fsux
}
