package fsux

import (
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/perf"
	"sigmaos/proc"
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
	ip, err := netsigma.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}
	fsux := newUx(rootux)
	root, sr := newDir([]string{rootux})
	if sr != nil {
		db.DFatalf("newDir %v\n", sr)
	}
	pe := proc.GetProcEnv()
	addr := sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, sp.NO_PORT)
	srv, err := sigmasrv.NewSigmaSrvRoot(root, path.Join(sp.UX, pe.GetKernelID()), addr, pe)
	if err != nil {
		db.DFatalf("BootSrvAndPost %v\n", err)
	}
	fsux.SigmaSrv = srv

	// Perf monitoring
	p, err := perf.NewPerf(pe, perf.UX)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	fsux.RunServer()
}

func newUx(rootux string) *FsUx {
	fsux = &FsUx{}
	fsux.ot = NewObjTable()
	return fsux
}
