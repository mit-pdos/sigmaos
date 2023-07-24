package sigmasrv

import (
	"runtime/debug"

	"sigmaos/cpumon"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/ephemeralmap"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/kernel"
	"sigmaos/memfs"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/protdev"
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
	sti    *protdev.StatInfo
	svc    *svcMap
	lsrv   *LeaseSrv
	cpumon *cpumon.CpuMon
}

// Make a sigmasrv with an memfs, and publish server at fn.
func MakeSigmaSrv(fn string, svci any, uname sp.Tuname) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFs(fn, uname)
	if error != nil {
		db.DFatalf("MakeSigmaSrv %v err %v\n", fn, error)
	}
	return MakeSigmaSrvMemFs(mfs, svci)
}

func MakeSigmaSrvPublic(fn string, svci any, uname sp.Tuname, public bool) (*SigmaSrv, error) {
	db.DPrintf(db.ALWAYS, "MakeSigmaSrvPublic %T\n", svci)
	if public {
		mfs, error := memfssrv.MakeMemFsPublic(fn, uname)
		if error != nil {
			return nil, error
		}
		return MakeSigmaSrvMemFs(mfs, svci)
	} else {
		return MakeSigmaSrv(fn, svci, uname)
	}
}

// Make a sigmasrv and memfs and publish srv at fn. Note: no lease
// server.
func MakeSigmaSrvNoRPC(fn string, uname sp.Tuname) (*SigmaSrv, error) {
	mfs, err := memfssrv.MakeMemFs(fn, uname)
	if err != nil {
		db.DFatalf("MakeSigmaSrv %v err %v\n", fn, err)
	}

	return newSigmaSrv(mfs), nil
}

func MakeSigmaSrvPort(fn, port string, uname sp.Tuname, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFsPort(fn, ":"+port, uname)
	if error != nil {
		db.DFatalf("MakeSigmaSrvPort %v err %v\n", fn, error)
	}
	return MakeSigmaSrvMemFs(mfs, svci)
}

func MakeSigmaSrvClnt(fn string, sc *sigmaclnt.SigmaClnt, uname sp.Tuname, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFsPortClnt(fn, ":0", sc)
	if error != nil {
		db.DFatalf("MakeSigmaSrvClnt %v err %v\n", fn, error)
	}
	return makeSigmaSrvRPC(mfs, svci)
}

func MakeSigmaSrvClntNoRPC(fn string, sc *sigmaclnt.SigmaClnt, uname sp.Tuname) (*SigmaSrv, error) {
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
	ssrv := &SigmaSrv{MemFs: mfs, svc: newSvcMap()}
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
	if _, err := ssrv.Create(protdev.RPC, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := ssrv.makeRPCDev(svci); err != nil {
		return err
	}
	return nil
}

func MakeSigmaSrvSess(sesssrv *sesssrv.SessSrv, uname sp.Tuname, sc *sigmaclnt.SigmaClnt) *SigmaSrv {
	mfs := memfssrv.MakeMemFsSrv(uname, "", sesssrv, sc)
	return newSigmaSrv(mfs)
}

func MakeSigmaSrvRoot(root fs.Dir, addr, path string, uname sp.Tuname) (*SigmaSrv, error) {
	sc, err := sigmaclnt.MkSigmaClnt(uname)
	if err != nil {
		return nil, err
	}
	et := ephemeralmap.NewEphemeralMap()
	sesssrv := fslibsrv.BootSrv(root, addr, nil, nil, et)
	ssrv := newSigmaSrv(memfssrv.MakeMemFsSrv(uname, "", sesssrv, sc))
	fslibsrv.Post(sesssrv, sc, path)
	return ssrv, nil
}

// Mount the rpc directory in sessrv and create the RPC service in
// it. This function is useful for SigmaSrv that don't have an MemFs
// (e.g., knamed/named).
func (ssrv *SigmaSrv) MountRPCSrv(svci any) error {
	d := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	ssrv.MemFs.SessSrv.Mount(protdev.RPC, d.(*dir.DirImpl))
	if err := ssrv.makeRPCDev(svci); err != nil {
		return err
	}
	return nil
}

// Make the rpc device and register the svci service
func (ssrv *SigmaSrv) makeRPCDev(svci any) error {
	ssrv.svc.RegisterService(svci)
	rd := mkRpcDev(ssrv)
	if err := sessdevsrv.MkSessDev(ssrv.MemFs, protdev.RPC, rd.mkRpcSession, nil); err != nil {
		return err
	}
	if si, err := makeStatsDev(ssrv.MemFs, protdev.RPC); err != nil {
		return err
	} else {
		ssrv.sti = si
	}
	return nil
}

// Assumes RPCSrv has been created and register the LeaseSrv service.
func (ssrv *SigmaSrv) NewLeaseSrv() error {
	ssrv.lsrv = newLeaseSrv(ssrv.MemFs)
	ssrv.svc.RegisterService(ssrv.lsrv)
	return nil
}

func (ssrv *SigmaSrv) QueueLen() int64 {
	return ssrv.MemFs.QueueLen()
}

func (ssrv *SigmaSrv) MonitorCPU(ufn cpumon.UtilFn) {
	ssrv.cpumon = cpumon.MkCpuMon(ssrv.GetStats(), ufn)
}

func (ssrv *SigmaSrv) RunServer() error {
	db.DPrintf(db.SIGMASRV, "Run %v\n", proc.GetProgram())
	ssrv.Serve()
	ssrv.Exit(proc.MakeStatus(proc.StatusEvicted))
	return nil
}

func (ssrv *SigmaSrv) Exit(status *proc.Status) error {
	db.DPrintf(db.SIGMASRV, "Exit %v\n", proc.GetProgram())
	if ssrv.lsrv != nil {
		ssrv.lsrv.Stop()
	}
	if ssrv.cpumon != nil {
		ssrv.cpumon.Done()
	}
	ssrv.MemFs.StopServing()
	return ssrv.MemFs.Exit(proc.MakeStatus(proc.StatusEvicted))
}

func (ssrv *SigmaSrv) Serve() {
	// If this is a kernel proc, register the subsystem info for the realmmgr
	if proc.GetIsPrivilegedProc() {
		si := kernel.MakeSubsystemInfo(proc.GetPid(), ssrv.MyAddr())
		kernel.RegisterSubsystemInfo(ssrv.MemFs.SigmaClnt().FsLib, si)
	}
	if err := ssrv.MemFs.SigmaClnt().Started(); err != nil {
		debug.PrintStack()
		db.DPrintf(db.ALWAYS, "Error Started: %v", err)
	}
	if err := ssrv.MemFs.SigmaClnt().WaitEvict(proc.GetPid()); err != nil {
		db.DPrintf(db.ALWAYS, "Error WaitEvict: %v", err)
	}
}
