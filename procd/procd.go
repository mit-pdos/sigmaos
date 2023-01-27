package procd

import (
	"path"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/procd/proto"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	scheddproto "sigmaos/schedd/proto"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/uprocclnt"
)

type Procd struct {
	mu             sync.Mutex
	fs             *ProcdFs
	realm          string                             // realm id of this procd
	kernelInitDone bool                               // True if kernel init has finished (this procd has spawned ux & s3).
	kernelProcs    map[string]bool                    // Map of kernel procs spawned on this procd.
	addr           string                             // Address of this procd.
	runningProcs   map[proc.Tpid]*LinuxProc           // Map of currently running procs.
	sigmaclnts     map[sp.Trealm]*sigmaclnt.SigmaClnt // Sigma clnt for each realm.
	coresAvail     proc.Tcore                         // Current number of cores available to run procs on.
	memAvail       proc.Tmem                          // Available memory for this procd and its procs to use.
	perf           *perf.Perf
	workers        sync.WaitGroup
	memfssrv       *memfssrv.MemFs
	schedd         *protdevclnt.ProtDevClnt
	updm           *uprocclnt.UprocdMgr
	pds            *protdevsrv.ProtDevSrv
	sc             *sigmaclnt.SigmaClnt
}

func RunProcd(realm string, spawningSys bool) {
	pd := &Procd{}
	pd.kernelProcs = make(map[string]bool)
	pd.kernelProcs["kernel/dbd"] = true
	// If we aren't spawning a full system on this procd, then kernel init is
	// done (this procd can start to accept procs).
	if !spawningSys {
		pd.kernelInitDone = true
	}
	pd.realm = realm
	pd.runningProcs = make(map[proc.Tpid]*LinuxProc)
	pd.sigmaclnts = make(map[sp.Trealm]*sigmaclnt.SigmaClnt)
	pd.coresAvail = proc.Tcore(linuxsched.NCores)
	pd.memAvail = mem.GetTotalMem()

	pd.makeFs()

	pd.addr = pd.memfssrv.MyAddr()
	var err error
	pd.schedd, err = protdevclnt.MkProtDevClnt(pd.sc.FsLib, path.Join(sp.SCHEDD, "~local"))
	if err != nil {
		db.DFatalf("Error make schedd clnt: %v", err)
	}

	pd.pds, err = protdevsrv.MakeProtDevSrvMemFs(pd.memfssrv, pd)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}

	perf, err := perf.MakePerf(perf.PROCD)
	if err != nil {
		db.DFatalf("MakePerf err %v", err)
	} else {
		pd.perf = perf
		defer pd.perf.Done()
	}

	// Make a directory in which to put stealable procs.
	pd.memfssrv.GetStats().DisablePathCnts()
	pd.memfssrv.GetStats().MonitorCPUUtil(pd.getLCProcUtil)
	pd.updm = uprocclnt.MakeUprocdMgr(pd.sc.FsLib)
	// Notify schedd that the proc is done running.
	req := &scheddproto.RegisterRequest{
		ProcdIp: pd.memfssrv.MyAddr(),
	}
	res := &scheddproto.RegisterResponse{}
	err = pd.schedd.RPC("Schedd.RegisterProcd", req, res)
	if err != nil {
		db.DFatalf("Error RegisterProcd schedd: %v", err)
	}

	pd.work()
}

func (pd *Procd) getLCProcUtil() float64 {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	var total float64 = 0.0
	for _, p := range pd.runningProcs {
		// If proc has not been initialized, or it isn't LC, move on
		if p.SysPid == 0 || p.attr.GetType() != proc.T_LC || p.attr.IsPrivilegedProc() {
			continue
		}
		u, err := p.getUtilL()
		if err != nil {
			db.DPrintf(db.PROCD_ERR, "getUtilL err %v\n", err)
			continue
		}
		total += u
	}
	return total
}

func (pd *Procd) getSigmaClnt(realm sp.Trealm) *sigmaclnt.SigmaClnt {
	var clnt *sigmaclnt.SigmaClnt
	var ok bool
	if clnt, ok = pd.sigmaclnts[realm]; !ok {
		// No need to make a new client for the root realm.
		if realm == sp.Trealm(proc.GetRealm()) {
			clnt = &sigmaclnt.SigmaClnt{pd.sc.FsLib, nil}
		} else {
			var err error
			if clnt, err = sigmaclnt.MkSigmaClntRealm(pd.sc.FsLib, sp.PROCDREL, realm); err != nil {
				db.DFatalf("Err MkSigmaClntRealm: %v", err)
			}
			// Mount KPIDS.
			procclnt.MountPids(clnt.FsLib, clnt.FsLib.NamedAddr())
		}
		pd.sigmaclnts[realm] = clnt
	}
	return clnt
}

// Caller holds lock.
func (pd *Procd) putProcL(p *LinuxProc) {
	pd.runningProcs[p.attr.GetPid()] = p
}

func (pd *Procd) deleteProc(p *LinuxProc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	delete(pd.runningProcs, p.attr.GetPid())
}

// Evict all procs running in this procd
func (pd *Procd) evictProcsL(procs map[proc.Tpid]*LinuxProc) {
	for pid, _ := range procs {
		pd.sc.EvictProcd(pd.addr, pid)
	}
}

func (pd *Procd) registerProcL(p *proc.Proc, stolen bool) *LinuxProc {
	// Create an ephemeral semaphore for the parent proc to wait on.
	sclnt := pd.getSigmaClnt(p.GetRealm())
	semPath := path.Join(p.ParentDir, proc.START_SEM)
	semStart := semclnt.MakeSemClnt(sclnt.FsLib, semPath)
	if err := semStart.Init(sp.DMTMP); err != nil {
		db.DPrintf(db.PROCD_ERR, "Error creating start semaphore path:%v err:%v", semPath, err)
	}
	db.DPrintf(db.PROCD, "Sem init done: %v", p)
	if err := pd.sc.Remove(path.Join(sp.SCHEDD, "~local", sp.QUEUE, p.GetPid().String())); err != nil {
		db.DFatalf("Error remove schedd file: %v", err)
	}
	if p.IsPrivilegedProc() && pd.kernelInitDone {
		db.DPrintf(db.PROCD, "Spawned privileged proc %v on fully initialized procd", p)
	}
	// Make a Linux Proc which corresponds to this proc.
	linuxProc := makeLinuxProc(pd, sclnt, p, stolen)
	// Allocate dedicated cores for this proc to run on, if it requires them.
	// Core allocation & resource accounting has to happen at this point, while
	// we still hold the lock we used to claim the proc, since more spawns may
	// happen at any time.
	pd.allocCoresL(p)
	pd.allocMemL(p)
	// Register running proc in in-memory structures.
	pd.putProcL(linuxProc)
	return linuxProc
}

// Returns true if the proc was there, but there was a capacity issue.
func (pd *Procd) runProc(p *LinuxProc) error {
	defer pd.workers.Done()
	if !p.attr.IsPrivilegedProc() {
		// Download the bin from s3, if it isn't already cached locally.
		if err := pd.downloadProcBin(p.attr); err != nil {
			db.DFatalf("runProc: failed to download proc %v\n", p.attr)
			return err
		}
	}
	// Run the proc.
	p.run()
	// Free any dedicated cores.
	pd.freeCores(p.attr)
	pd.freeMem(p.attr)
	// Deregister running procs
	pd.deleteProc(p)
	// Notify schedd that the proc is done running.
	req := &scheddproto.ProcDoneRequest{
		ProcProto: p.attr.GetProto(),
	}
	res := &scheddproto.ProcDoneResponse{}
	err := pd.schedd.RPC("Schedd.ProcDone", req, res)
	if err != nil {
		db.DFatalf("Error ProcDone schedd: %v", err)
		return err
	}
	return nil
}

// Run a proc.
func (pd *Procd) RunProc(req proto.RunProcRequest, res *proto.RunProcResponse) error {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	p := proc.MakeProcFromProto(req.ProcProto)
	db.DPrintf(db.PROCD, "Got proc %v", p)
	linuxProc := pd.registerProcL(p, false)
	err := pd.fs.running(linuxProc)
	if err != nil {
		pd.perf.Done()
		db.DFatalf("Procd pub running error %v %T\n", err, err)
	}
	// Run this proc in a separate thread.
	pd.workers.Add(1)
	go pd.runProc(linuxProc)
	return nil
}

func (pd *Procd) work() {
	db.DPrintf(db.PROCD, "Work")
	pd.workers.Add(1)
	go func() {
		defer pd.workers.Done()
		pd.memfssrv.Serve()
		pd.memfssrv.Done()
	}()
	pd.workers.Wait()
}
