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
	"sigmaos/kernel"
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
func MakeSigmaSrv(fn string, svci any, pcfg *proc.ProcEnv) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFs(fn, pcfg)
	if error != nil {
		db.DFatalf("MakeSigmaSrv %v err %v\n", fn, error)
	}
	return MakeSigmaSrvMemFs(mfs, svci)
}

func MakeSigmaSrvPublic(fn string, svci any, pcfg *proc.ProcEnv, public bool) (*SigmaSrv, error) {
	db.DPrintf(db.ALWAYS, "MakeSigmaSrvPublic %T\n", svci)
	if public {
		mfs, error := memfssrv.MakeMemFsPublic(fn, pcfg)
		if error != nil {
			return nil, error
		}
		return MakeSigmaSrvMemFs(mfs, svci)
	} else {
		return MakeSigmaSrv(fn, svci, pcfg)
	}
}

// Make a sigmasrv and memfs and publish srv at fn. Note: no lease
// server.
func MakeSigmaSrvNoRPC(fn string, pcfg *proc.ProcEnv) (*SigmaSrv, error) {
	mfs, err := memfssrv.MakeMemFs(fn, pcfg)
	if err != nil {
		db.DFatalf("MakeSigmaSrv %v err %v\n", fn, err)
	}
	return newSigmaSrv(mfs), nil
}

func MakeSigmaSrvPort(fn, port string, pcfg *proc.ProcEnv, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFsPort(fn, ":"+port, pcfg)
	if error != nil {
		db.DFatalf("MakeSigmaSrvPort %v err %v\n", fn, error)
	}
	return MakeSigmaSrvMemFs(mfs, svci)
}

func MakeSigmaSrvClnt(fn string, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFsPortClnt(fn, ":0", sc)
	if error != nil {
		db.DFatalf("MakeSigmaSrvClnt %v err %v\n", fn, error)
	}
	return makeSigmaSrvRPC(mfs, svci)
}

func MakeSigmaSrvClntFence(fn string, sc *sigmaclnt.SigmaClnt, svci any) (*SigmaSrv, error) {
	ffs := fencefs.MakeRoot(ctx.MkCtxNull(), nil)
	mfs, error := memfssrv.MakeMemFsPortClntFence(fn, ":0", sc, ffs)
	if error != nil {
		db.DFatalf("MakeSigmaSrvClnt %v err %v\n", fn, error)
	}
	mfs.Mount(sp.FENCEDIR, ffs.(*dir.DirImpl))
	return makeSigmaSrvRPC(mfs, svci)
}

func MakeSigmaSrvClntNoRPC(fn string, sc *sigmaclnt.SigmaClnt) (*SigmaSrv, error) {
	mfs, err := memfssrv.MakeMemFsPortClnt(fn, ":0", sc)
	if err != nil {
		db.DFatalf("MakeMemFsPortClnt %v err %v\n", fn, err)
	}
	ssrv := newSigmaSrv(mfs)
	return ssrv, nil
}

// Makes a sigmasrv with an memfs, rpc server, and LeaseSrv service.
func MakeSigmaSrvMemFs(mfs *memfssrv.MemFs, svci any) (*SigmaSrv, error) {
	ssrv, err := makeSigmaSrvRPC(mfs, svci)
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
func makeSigmaSrvRPC(mfs *memfssrv.MemFs, svci any) (*SigmaSrv, error) {
	ssrv := newSigmaSrv(mfs)
	return ssrv, ssrv.makeRPCSrv(svci)
}

// Create the rpc server directory in memfs and make the RPC dev and
// register svci.
func (ssrv *SigmaSrv) makeRPCSrv(svci any) error {
	db.DPrintf(db.SIGMASRV, "makeRPCSrv: %v\n", svci)
	if _, err := ssrv.Create(rpc.RPC, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := ssrv.makeRPCDev(svci); err != nil {
		return err
	}
	return nil
}

func MakeSigmaSrvSess(sesssrv *sesssrv.SessSrv, uname sp.Tuname, sc *sigmaclnt.SigmaClnt) *SigmaSrv {
	mfs := memfssrv.NewMemFsSrv("", sesssrv, sc, nil)
	return newSigmaSrv(mfs)
}

func MakeSigmaSrvRoot(root fs.Dir, addr, path string, pcfg *proc.ProcEnv) (*SigmaSrv, error) {
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
	d := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	ssrv.MemFs.SessSrv.Mount(rpc.RPC, d.(*dir.DirImpl))
	if err := ssrv.makeRPCDev(svci); err != nil {
		return err
	}
	return nil
}

// Make the rpc device and register the svci service
func (ssrv *SigmaSrv) makeRPCDev(svci any) error {
	if si, err := makeStatsDev(ssrv.MemFs, rpc.RPC); err != nil {
		return err
	} else {
		ssrv.rpcs = rpcsrv.NewRPCSrv(svci, si)
		rd := mkRpcDev(ssrv.rpcs)
		if err := sessdevsrv.MkSessDev(ssrv.MemFs, rpc.RPC, rd.mkRpcSession, nil); err != nil {
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
	ssrv.cpumon = cpumon.MkCpuMon(ssrv.GetStats(), ufn)
}

func (ssrv *SigmaSrv) RunServer() error {
	db.DPrintf(db.SIGMASRV, "Run %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	ssrv.Serve()
	ssrv.SrvExit(proc.MakeStatus(proc.StatusEvicted))
	return nil
}

func (ssrv *SigmaSrv) SrvExit(status *proc.Status) error {
	db.DPrintf(db.SIGMASRV, "SrvExit %v", ssrv.MemFs.SigmaClnt().ProcEnv().Program)
	if ssrv.lsrv != nil {
		ssrv.lsrv.stop()
	}
	if ssrv.cpumon != nil {
		ssrv.cpumon.Done()
	}
	ssrv.MemFs.StopServing()
	return ssrv.MemFs.MemFsExit(proc.MakeStatus(proc.StatusEvicted))
}

func (ssrv *SigmaSrv) Serve() {
	// If this is a kernel proc, register the subsystem info for the realmmgr
	if ssrv.SigmaClnt().ProcEnv().Privileged {
		si := kernel.MakeSubsystemInfo(ssrv.SigmaClnt().ProcEnv().GetPID(), ssrv.MyAddr())
		kernel.RegisterSubsystemInfo(ssrv.MemFs.SigmaClnt().FsLib, si)
	}
	if err := ssrv.MemFs.SigmaClnt().Started(); err != nil {
		debug.PrintStack()
		db.DPrintf(db.ALWAYS, "Error Started: %v", err)
	}
	if err := ssrv.MemFs.SigmaClnt().WaitEvict(ssrv.SigmaClnt().ProcEnv().GetPID()); err != nil {
		db.DPrintf(db.ALWAYS, "Error WaitEvict: %v", err)
	}
}
