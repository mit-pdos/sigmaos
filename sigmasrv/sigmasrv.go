// Many SigmaOS servers use package sigmasrv to create and run
// servers.  A server typically consists of a MemFS (an in-memory file
// system accessed through sigmap), one or more RPC services,
// including one for leases. Sigmasrv creates the RPC device in the
// memfs.
package sigmasrv

import (
	"runtime/debug"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/fencefs"
	"sigmaos/api/fs"
	"sigmaos/memfs"
	"sigmaos/memfs/dir"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/rpc"
	rpcsrv "sigmaos/rpc/srv"
	rpcdevsrv "sigmaos/rpc/dev/srv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/cpumon"
	spprotosrv "sigmaos/spproto/srv"
)

type SigmaSrv struct {
	*memfssrv.MemFs
	rpcs   *rpcsrv.RPCSrv
	lsrv   *LeaseSrv
	cpumon *cpumon.CpuMon
}

// Make a sigmasrv with an memfs, and publish server at fn.
func NewSigmaSrv(fn string, svci any, pe *proc.ProcEnv) (*SigmaSrv, error) {
	db.DPrintf(db.SIGMASRV, "NewSigmaSrv %T", svci)
	defer db.DPrintf(db.SIGMASRV, "NewSigmaSrv done %T", svci)

	mfs, error := memfssrv.NewMemFs(fn, pe, spprotosrv.AttachAllowAllToAll)
	if error != nil {
		db.DPrintf(db.ERROR, "NewSigmaSrv %v err %v", fn, error)
		return nil, error
	}
	return newSigmaSrvMemFs(mfs, svci)
}

func NewSigmaSrvAddrClnt(fn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFsAddrClnt(fn, addr, sc, spprotosrv.AttachAllowAllToAll)
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
	return NewSigmaSrvClntAuthFn(fn, sc, svci, spprotosrv.AttachAllowAllToAll)
}

func NewSigmaSrvClntAuthFn(fn string, sc *sigmaclnt.SigmaClnt, svci any, aaf spprotosrv.AttachAuthF) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFsPortClnt(fn, sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP), sc, aaf)
	if error != nil {
		db.DPrintf(db.ERROR, "NewSigmaSrvClnt %v err %v", fn, error)
		return nil, error
	}
	return newSigmaSrvMemFs(mfs, svci)
}

// For an memfs server: memfs, lease srv, and fences
func NewSigmaSrvClntFence(fn string, sc *sigmaclnt.SigmaClnt) (*SigmaSrv, error) {
	ffs := fencefs.NewRoot(ctx.NewCtxNull(), nil)
	mfs, error := memfssrv.NewMemFsPortClntFenceAuth(fn, sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP), sc, ffs, spprotosrv.AttachAllowAllToAll)
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
	mfs, err := memfssrv.NewMemFsPortClnt(fn, sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP), sc, spprotosrv.AttachAllowAllToAll)
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

func NewSigmaSrvRootClnt(root fs.Dir, addr *sp.Taddr, path string, sc *sigmaclnt.SigmaClnt) (*SigmaSrv, error) {
	return NewSigmaSrvRootClntAuthFn(root, addr, path, sc, spprotosrv.AttachAllowAllToAll)
}

func NewSigmaSrvRootClntAuthFn(root fs.Dir, addr *sp.Taddr, path string, sc *sigmaclnt.SigmaClnt, aaf spprotosrv.AttachAuthF) (*SigmaSrv, error) {
	mfs, err := memfssrv.NewMemFsRootPortClntFenceAuth(root, path, addr, sc, nil, aaf)
	if err != nil {
		return nil, err
	}
	return newSigmaSrv(mfs), nil
}

func NewSigmaSrvRoot(root fs.Dir, path string, addr *sp.Taddr, pe *proc.ProcEnv) (*SigmaSrv, error) {
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	return NewSigmaSrvRootClnt(root, addr, path, sc)
}

// Mount the rpc directory and create the RPC service in it. This
// function is useful for sigmasrv whose root is for a non-memfs file
// system (e.g., knamed/named, which uses fsetcd).
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
		if err := rpcdevsrv.NewSessDev(ssrv.MemFs, relpath, rd.newRpcSession, nil); err != nil {
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
