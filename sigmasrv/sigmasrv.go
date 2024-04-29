// Many SigmaOS servers use package sigmasrv to create and run
// servers.  A server typically consists of a MemFS (an in-memory file
// system accessed through sigmap), one or more RPC services,
// including one for leases. Sigmasrv creates the RPC device in the
// memfs.
package sigmasrv

import (
	"runtime/debug"

	"sigmaos/auth"
	"sigmaos/cpumon"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fencefs"
	"sigmaos/fs"
	"sigmaos/memfs"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/rpcsrv"
	"sigmaos/sessdevsrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type SigmaSrv struct {
	*memfssrv.MemFs
	rpcs   *rpcsrv.RPCSrv
	lsrv   *LeaseSrv
	cpumon *cpumon.CpuMon
}

// Make a sigmasrv with an memfs, and publish server at fn.
func NewSigmaSrv(fn string, svci any, pe *proc.ProcEnv) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFs(fn, pe)
	if error != nil {
		db.DPrintf(db.ERROR, "NewSigmaSrv %v err %v", fn, error)
		return nil, error
	}
	return newSigmaSrvMemFs(mfs, svci)
}

func NewSigmaSrvPublic(fn string, svci any, pe *proc.ProcEnv, public bool) (*SigmaSrv, error) {
	db.DPrintf(db.SIGMASRV, "NewSigmaSrvPublic %T", svci)
	defer db.DPrintf(db.SIGMASRV, "NewSigmaSrvPublic done %T", svci)

	if public {
		mfs, error := memfssrv.NewMemFsPublic(fn, pe)
		if error != nil {
			db.DPrintf(db.ERROR, "NewMemFsPublic %v err %v", fn, error)
			return nil, error
		}
		return newSigmaSrvMemFs(mfs, svci)
	} else {
		return NewSigmaSrv(fn, svci, pe)
	}
}

func NewSigmaSrvAddrClnt(fn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFsAddrClnt(fn, addr, sc)
	if error != nil {
		db.DPrintf(db.ERROR, "NewSigmaSrvPort %v err %v", fn, error)
		return nil, error
	}
	return newSigmaSrvMemFs(mfs, svci)
}

func NewSigmaSrvAddr(fn string, addr *sp.Taddr, pe *proc.ProcEnv, svci any) (*SigmaSrv, error) {
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	return NewSigmaSrvAddrClnt(fn, addr, sc, svci)
}

func NewSigmaSrvClnt(fn string, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFsPortClnt(fn, sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP, sc.ProcEnv().GetNet()), sc)
	if error != nil {
		db.DPrintf(db.ERROR, "NewSigmaSrvClnt %v err %v", fn, error)
		return nil, error
	}
	return newSigmaSrvMemFs(mfs, svci)
}

func NewSigmaSrvClntKeyMgr(fn string, sc *sigmaclnt.SigmaClnt, kmgr auth.KeyMgr, svci any) (*SigmaSrv, error) {
	mfs, err := memfssrv.NewMemFsPortClntFenceKeyMgr(fn, sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP, sc.ProcEnv().GetNet()), sc, nil, kmgr)
	if err != nil {
		db.DPrintf(db.ERROR, "NewSigmaSrvClnt %v err %v", fn, err)
		return nil, err
	}
	return newSigmaSrvMemFs(mfs, svci)
}

// For an memfs server: memfs, lease srv, and fences
func NewSigmaSrvClntFence(fn string, sc *sigmaclnt.SigmaClnt) (*SigmaSrv, error) {
	ffs := fencefs.NewRoot(ctx.NewCtxNull(), nil)
	mfs, error := memfssrv.NewMemFsPortClntFence(fn, sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP, sc.ProcEnv().GetNet()), sc, ffs)
	if error != nil {
		db.DPrintf(db.ERROR, "NewSigmaSrvClntFence %v err %v", fn, error)
		return nil, error
	}
	mfs.Mount(sp.FENCEDIR, ffs.(*dir.DirImpl))
	lsrv := newLeaseSrv(mfs)
	ssrv, err := newSigmaSrvRPC(mfs, lsrv)
	if err != nil {
		return nil, err
	}
	ssrv.lsrv = lsrv
	return ssrv, nil
}

func NewSigmaSrvClntNoRPC(fn string, sc *sigmaclnt.SigmaClnt) (*SigmaSrv, error) {
	mfs, err := memfssrv.NewMemFsPortClnt(fn, sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP, sc.ProcEnv().GetNet()), sc)
	if err != nil {
		db.DPrintf(db.ERROR, "NewMemFsPortClnt %v err %v", fn, err)
		return nil, err
	}
	ssrv := newSigmaSrv(mfs)
	return ssrv, nil
}

// Create the rpc server directory in memfs and make the RPC dev and
// register svci.
func (ssrv *SigmaSrv) AddRPCSrv(relpath string, svci any) error {
	db.DPrintf(db.SIGMASRV, "newRPCSrv: %v", svci)
	if _, err := ssrv.Create(relpath, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := ssrv.newRPCDev(relpath, svci); err != nil {
		return err
	}
	return nil
}

// Creates a sigmasrv with an memfs, rpc server, and LeaseSrv service.
func newSigmaSrvMemFs(mfs *memfssrv.MemFs, svci any) (*SigmaSrv, error) {
	ssrv, err := newSigmaSrvRPC(mfs, svci)
	if err != nil {
		return nil, err
	}
	ssrv.newLeaseSrv()
	return ssrv, nil
}

func newSigmaSrv(mfs *memfssrv.MemFs) *SigmaSrv {
	ssrv := &SigmaSrv{MemFs: mfs}
	return ssrv
}

// Make a sigmasrv with an RPC server
func newSigmaSrvRPC(mfs *memfssrv.MemFs, svci any) (*SigmaSrv, error) {
	ssrv := newSigmaSrv(mfs)
	return ssrv, ssrv.AddRPCSrv(rpc.RPC, svci)
}

func NewSigmaSrvRootClntKeyMgr(root fs.Dir, addr *sp.Taddr, path string, sc *sigmaclnt.SigmaClnt, keymgr auth.KeyMgr) (*SigmaSrv, error) {
	mfs, err := memfssrv.NewMemFsRootPortClntFenceKeyMgr(root, path, addr, sc, keymgr, nil)
	if err != nil {
		return nil, err
	}
	return newSigmaSrv(mfs), nil
}

func NewSigmaSrvRootClnt(root fs.Dir, path string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt) (*SigmaSrv, error) {
	return NewSigmaSrvRootClntKeyMgr(root, addr, path, sc, nil)
}

func NewSigmaSrvRoot(root fs.Dir, path string, addr *sp.Taddr, pe *proc.ProcEnv) (*SigmaSrv, error) {
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	return NewSigmaSrvRootClnt(root, path, addr, sc)
}

// Mount the rpc directory in sessrv and create the RPC service in
// it. This function is useful for SigmaSrv that don't have an MemFs
// (e.g., knamed/named).
func (ssrv *SigmaSrv) MountRPCSrv(svci any) error {
	d := dir.NewRootDir(ctx.NewCtxNull(), memfs.NewInode, nil)
	ssrv.MemFs.SigmaPSrv.Mount(rpc.RPC, d.(*dir.DirImpl))
	if err := ssrv.newRPCDev(rpc.RPC, svci); err != nil {
		return err
	}
	return nil
}

// Make the rpc device and register the svci service
func (ssrv *SigmaSrv) newRPCDev(relpath string, svci any) error {
	if si, err := newStatsDev(ssrv.MemFs, relpath); err != nil {
		return err
	} else {
		ssrv.rpcs = rpcsrv.NewRPCSrv(svci, si)
		rd := newRpcDev(ssrv.rpcs)
		if err := sessdevsrv.NewSessDev(ssrv.MemFs, relpath, rd.newRpcSession, nil); err != nil {
			return err
		}
		return nil
	}
}

// Assumes RPCSrv has been created and register the LeaseSrv service.
func (ssrv *SigmaSrv) newLeaseSrv() {
	ssrv.lsrv = newLeaseSrv(ssrv.MemFs)
	ssrv.rpcs.RegisterService(ssrv.lsrv)
}

func (ssrv *SigmaSrv) QueueLen() int64 {
	return ssrv.MemFs.QueueLen()
}

func (ssrv *SigmaSrv) MonitorCPU(ufn cpumon.UtilFn) {
	ssrv.cpumon = cpumon.NewCpuMon(ssrv.MemFs.Stats(), ufn)
}

func (ssrv *SigmaSrv) RunServer() error {
	db.DPrintf(db.SIGMASRV, "Run %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	ssrv.Serve()
	return ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))
}

func (ssrv *SigmaSrv) SrvExit(status *proc.Status) error {
	db.DPrintf(db.SIGMASRV, "SrvExit %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	defer db.DPrintf(db.SIGMASRV, "SrvExit done %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	if ssrv.lsrv != nil {
		ssrv.lsrv.stop()
	}
	db.DPrintf(db.SIGMASRV, "lsrv stop %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	if ssrv.cpumon != nil {
		ssrv.cpumon.Done()
	}
	db.DPrintf(db.SIGMASRV, "cpumon done %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	ssrv.MemFs.StopServing()
	db.DPrintf(db.ALWAYS, "StopServing %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	return ssrv.MemFs.MemFsExit(proc.NewStatus(proc.StatusEvicted))
}

func (ssrv *SigmaSrv) Serve() {
	if err := ssrv.MemFs.SigmaClnt().Started(); err != nil {
		debug.PrintStack()
		db.DPrintf(db.ALWAYS, "Error Started: %v", err)
	}
	if err := ssrv.MemFs.SigmaClnt().WaitEvict(ssrv.SigmaClnt().ProcEnv().GetPID()); err != nil {
		db.DPrintf(db.ALWAYS, "Error WaitEvict: %v", err)
	}
}
