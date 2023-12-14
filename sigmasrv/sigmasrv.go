package sigmasrv

import (
	"runtime/debug"

	"sigmaos/cpumon"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/ephemeralmap"
	"sigmaos/fencefs"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/kernelsubinfo"
	"sigmaos/memfs"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/rpcsrv"
	"sigmaos/sessdevsrv"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Many SigmaOS servers use SigmaSrv to create and run servers.  A
// server typically consists of a MemFS (an in-memory file system
// accessed through sigmap), one or more RPC services, including one
// for leases. Sigmasrv creates the RPC device in the memfs.
//

type SigmaSrv struct {
	*memfssrv.MemFs
	rpcs   *rpcsrv.RPCSrv
	lsrv   *LeaseSrv
	cpumon *cpumon.CpuMon
}

// Make a sigmasrv with an memfs, and publish server at fn.
func NewSigmaSrv(fn string, svci any, pcfg *proc.ProcEnv) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFs(fn, pcfg)
	if error != nil {
		db.DFatalf("NewSigmaSrv %v err %v\n", fn, error)
	}
	return newSigmaSrvMemFs(mfs, svci)
}

func NewSigmaSrvPublic(fn string, svci any, pcfg *proc.ProcEnv, public bool) (*SigmaSrv, error) {
	db.DPrintf(db.ALWAYS, "NewSigmaSrvPublic %T\n", svci)
	if public {
		mfs, error := memfssrv.NewMemFsPublic(fn, pcfg)
		if error != nil {
			return nil, error
		}
		return newSigmaSrvMemFs(mfs, svci)
	} else {
		return NewSigmaSrv(fn, svci, pcfg)
	}
}

func NewSigmaSrvPort(fn, port string, pcfg *proc.ProcEnv, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFsPort(fn, ":"+port, pcfg)
	if error != nil {
		db.DFatalf("NewSigmaSrvPort %v err %v\n", fn, error)
	}
	return newSigmaSrvMemFs(mfs, svci)
}

func NewSigmaSrvClnt(fn string, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFsPortClnt(fn, ":0", sc)
	if error != nil {
		db.DFatalf("NewSigmaSrvClnt %v err %v\n", fn, error)
	}
	return newSigmaSrvRPC(mfs, svci)
}

func NewSigmaSrvClntLease(fn string, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.NewMemFsPortClnt(fn, ":0", sc)
	if error != nil {
		db.DFatalf("NewSigmaSrvClnt %v err %v\n", fn, error)
	}
	return newSigmaSrvMemFs(mfs, svci)
}

func NewSigmaSrvClntFence(fn string, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	ffs := fencefs.NewRoot(ctx.NewCtxNull(), nil)
	mfs, error := memfssrv.NewMemFsPortClntFence(fn, ":0", sc, ffs)
	if error != nil {
		db.DFatalf("NewSigmaSrvClnt %v err %v\n", fn, error)
	}
	mfs.Mount(sp.FENCEDIR, ffs.(*dir.DirImpl))
	return newSigmaSrvRPC(mfs, svci)
}

func NewSigmaSrvClntNoRPC(fn string, sc *sigmaclnt.SigmaClnt) (*SigmaSrv, error) {
	mfs, err := memfssrv.NewMemFsPortClnt(fn, ":0", sc)
	if err != nil {
		db.DFatalf("NewMemFsPortClnt %v err %v\n", fn, err)
	}
	ssrv := newSigmaSrv(mfs)
	return ssrv, nil
}

// Creates a sigmasrv with an memfs, rpc server, and LeaseSrv service.
func newSigmaSrvMemFs(mfs *memfssrv.MemFs, svci any) (*SigmaSrv, error) {
	ssrv, err := newSigmaSrvRPC(mfs, svci)
	if err != nil {
		return nil, err
	}
	if err := ssrv.NewLeaseSrv(); err != nil {
		return nil, err
	}
	return ssrv, nil
}

func newSigmaSrv(mfs *memfssrv.MemFs) *SigmaSrv {
	ssrv := &SigmaSrv{MemFs: mfs}
	return ssrv
}

// Make a sigmasrv with an RPC server
func newSigmaSrvRPC(mfs *memfssrv.MemFs, svci any) (*SigmaSrv, error) {
	ssrv := newSigmaSrv(mfs)
	return ssrv, ssrv.newRPCSrv(svci)
}

// Create the rpc server directory in memfs and make the RPC dev and
// register svci.
func (ssrv *SigmaSrv) newRPCSrv(svci any) error {
	db.DPrintf(db.SIGMASRV, "newRPCSrv: %v\n", svci)
	if _, err := ssrv.Create(rpc.RPC, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := ssrv.newRPCDev(svci); err != nil {
		return err
	}
	return nil
}

func NewSigmaSrvSess(sesssrv *sesssrv.SessSrv, sc *sigmaclnt.SigmaClnt) *SigmaSrv {
	mfs := memfssrv.NewMemFsSrv("", sesssrv, sc, nil)
	return newSigmaSrv(mfs)
}

func NewSigmaSrvRoot(root fs.Dir, addr, path string, pcfg *proc.ProcEnv) (*SigmaSrv, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	et := ephemeralmap.NewEphemeralMap()
	sesssrv := fslibsrv.BootSrv(sc.ProcEnv(), root, addr, nil, nil, et)
	ssrv := newSigmaSrv(memfssrv.NewMemFsSrv("", sesssrv, sc, nil))
	fslibsrv.Post(sesssrv, sc, path)
	return ssrv, nil
}

// Mount the rpc directory in sessrv and create the RPC service in
// it. This function is useful for SigmaSrv that don't have an MemFs
// (e.g., knamed/named).
func (ssrv *SigmaSrv) MountRPCSrv(svci any) error {
	d := dir.NewRootDir(ctx.NewCtxNull(), memfs.NewInode, nil)
	ssrv.MemFs.SessSrv.Mount(rpc.RPC, d.(*dir.DirImpl))
	if err := ssrv.newRPCDev(svci); err != nil {
		return err
	}
	return nil
}

// Make the rpc device and register the svci service
func (ssrv *SigmaSrv) newRPCDev(svci any) error {
	if si, err := newStatsDev(ssrv.MemFs, rpc.RPC); err != nil {
		return err
	} else {
		ssrv.rpcs = rpcsrv.NewRPCSrv(svci, si)
		rd := newRpcDev(ssrv.rpcs)
		if err := sessdevsrv.NewSessDev(ssrv.MemFs, rpc.RPC, rd.newRpcSession, nil); err != nil {
			return err
		}
		return nil
	}
}

// Assumes RPCSrv has been created and register the LeaseSrv service.
func (ssrv *SigmaSrv) NewLeaseSrv() error {
	ssrv.lsrv = newLeaseSrv(ssrv.MemFs)
	ssrv.rpcs.RegisterService(ssrv.lsrv)
	return nil
}

func (ssrv *SigmaSrv) QueueLen() int64 {
	return ssrv.MemFs.QueueLen()
}

func (ssrv *SigmaSrv) MonitorCPU(ufn cpumon.UtilFn) {
	ssrv.cpumon = cpumon.NewCpuMon(ssrv.GetStats(), ufn)
}

func (ssrv *SigmaSrv) RunServer() error {
	db.DPrintf(db.SIGMASRV, "Run %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	ssrv.Serve()
	ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))
	return nil
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
	db.DPrintf(db.SIGMASRV, "StopServing %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	return ssrv.MemFs.MemFsExit(proc.NewStatus(proc.StatusEvicted))
}

func (ssrv *SigmaSrv) Serve() {
	// If this is a kernel proc, register the subsystem info for the realmmgr
	if ssrv.SigmaClnt().ProcEnv().Privileged {
		si := kernelsubinfo.NewSubsystemInfo(ssrv.SigmaClnt().ProcEnv().GetPID(), ssrv.MyAddr())
		kernelsubinfo.RegisterSubsystemInfo(ssrv.MemFs.SigmaClnt().FsLib, si)
	}
	if err := ssrv.MemFs.SigmaClnt().Started(); err != nil {
		debug.PrintStack()
		db.DPrintf(db.ALWAYS, "Error Started: %v", err)
	}
	if err := ssrv.MemFs.SigmaClnt().WaitEvict(ssrv.SigmaClnt().ProcEnv().GetPID()); err != nil {
		db.DPrintf(db.ALWAYS, "Error WaitEvict: %v", err)
	}
}
