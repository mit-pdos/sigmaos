package fsux

import (
	"sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/leasemgrsrv"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/repl"
	sp "sigmaos/sigmap"
	// "sigmaos/seccomp"
)

var fsux *FsUx

type FsUx struct {
	*memfssrv.MemFs
	mount string

	sync.Mutex
	ot *ObjTable
}

func RunFsUx(rootux string) {
	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}
	// XXX make mfs?
	fsux = MakeReplicatedFsUx(rootux, ip+":0", proc.GetPid(), nil)
	fsux.Serve()
	fsux.Exit(proc.MakeStatus(proc.StatusEvicted))
}

func MakeReplicatedFsUx(rootux string, addr string, pid proc.Tpid, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux = &FsUx{}
	fsux.ot = MkObjTable()
	root, err := makeDir([]string{rootux})
	if err != nil {
		db.DFatalf("%v: makeDir %v\n", proc.GetName(), err)
	}
	mfs, error := memfssrv.MakeMemFsReplServer(root, addr, sp.UX, "ux", config)
	if error != nil {
		db.DFatalf("%v: MakeReplServer %v\n", proc.GetName(), error)
	}
	lsrv := memfssrv.NewLeaseSrv(mfs)
	_, error = leasemgrsrv.NewLeaseMgrSrv(sp.Tuname(addr), mfs.SessSrv, lsrv)
	if error != nil {
		db.DFatalf("%v: NewLeaseMgrSrv %v\n", proc.GetName(), error)
	}
	fsux.MemFs = mfs
	return fsux
}
