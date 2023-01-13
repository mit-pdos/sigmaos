package procd

import (
	"path"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
	scheddproto "sigmaos/schedd/proto"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
)

type Procd struct {
	sync.Mutex
	fs             *ProcdFs
	realm          string                   // realm id of this procd
	done           bool                     // Finished running.
	kernelInitDone bool                     // True if kernel init has finished (this procd has spawned ux & s3).
	kernelProcs    map[string]bool          // Map of kernel procs spawned on this procd.
	addr           string                   // Address of this procd.
	runningProcs   map[proc.Tpid]*LinuxProc // Map of currently running procs.
	coresAvail     proc.Tcore               // Current number of cores available to run procs on.
	memAvail       proc.Tmem                // Available memory for this procd and its procs to use.
	perf           *perf.Perf
	workers        sync.WaitGroup
	procclnt       *procclnt.ProcClnt
	memfssrv       *memfssrv.MemFs
	schedd         *protdevclnt.ProtDevClnt
	*fslib.FsLib
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
	pd.coresAvail = proc.Tcore(linuxsched.NCores)
	pd.memAvail = getMemTotal()

	pd.makeFs()

	pd.addr = pd.memfssrv.MyAddr()
	var err error
	pd.schedd, err = protdevclnt.MkProtDevClnt(pd.FsLib, path.Join(sp.SCHEDD, "~local"))
	if err != nil {
		db.DFatalf("Error make schedd clnt: %v", err)
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

	pd.Work()
}

func (pd *Procd) getLCProcUtil() float64 {
	pd.Lock()
	defer pd.Unlock()
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

// Caller holds lock.
func (pd *Procd) putProcL(p *LinuxProc) {
	pd.runningProcs[p.attr.GetPid()] = p
}

func (pd *Procd) deleteProc(p *LinuxProc) {
	pd.Lock()
	defer pd.Unlock()
	delete(pd.runningProcs, p.attr.GetPid())
}

// Evict all procs running in this procd
func (pd *Procd) evictProcsL(procs map[proc.Tpid]*LinuxProc) {
	for pid, _ := range procs {
		pd.procclnt.EvictProcd(pd.addr, pid)
	}
}

func (pd *Procd) Done() {
	pd.Lock()
	defer pd.Unlock()

	pd.done = true
	pd.perf.Done()
	pd.evictProcsL(pd.runningProcs)
}

func (pd *Procd) readDone() bool {
	pd.Lock()
	defer pd.Unlock()
	return pd.done
}

func (pd *Procd) registerProcL(p *proc.Proc, stolen bool) *LinuxProc {
	if p.IsPrivilegedProc() && pd.kernelInitDone {
		db.DPrintf(db.ALWAYS, "Spawned privileged proc %v on fully initialized procd", p)
	}
	// Make a Linux Proc which corresponds to this proc.
	linuxProc := makeLinuxProc(pd, p, stolen)
	// Allocate dedicated cores for this proc to run on, if it requires them.
	// Core allocation & resource accounting has to happen at this point, while
	// we still hold the lock we used to claim the proc, since this procd may be
	// resized at any time. When the resize happens, we *must* have already
	// assigned cores to this proc & registered it in the procd in-mem data
	// structures so that the proc's core allocations will be adjusted during the
	// resize.
	pd.allocCoresL(p.GetNcore())
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
			db.DFatalf("runProc: failed to download proc %v\n", p)
			return err
		}
	}

	// Run the proc.
	p.run()

	// Free any dedicated cores.
	pd.freeCores(p)
	pd.freeMem(p.attr)

	// Deregister running procs
	pd.deleteProc(p)
	return nil
}

func (pd *Procd) claimProc(p *proc.Proc, procPath string) bool {
	// Create an ephemeral semaphore for the parent proc to wait on.
	semStart := semclnt.MakeSemClnt(pd.FsLib, path.Join(p.ParentDir, proc.START_SEM))
	if err := semStart.Init(sp.DMTMP); err != nil {
		db.DFatalf("Error creating start semaphore: %v", err)
	}
	db.DPrintf(db.PROCD, "Sem init done: %v", p)
	if err := pd.Remove(path.Join(sp.SCHEDD, "~local", sp.QUEUE, p.GetPid().String())); err != nil {
		db.DFatalf("Error remove schedd file: %v", err)
	}
	return true
}

func (pd *Procd) getProc() (*LinuxProc, bool) {
	// TODO: get proc from schedd.
	req := &scheddproto.GetProcRequest{
		FreeCores: uint32(pd.coresAvail), // XXX fix race
		Mem:       uint32(pd.memAvail),   // XXX fix race
	}
	res := &scheddproto.GetProcResponse{}
	err := pd.schedd.RPC("Schedd.GetProc", req, res)
	if err != nil {
		db.DFatalf("Error getProc schedd: %v", err)
		return nil, false
	}
	if !res.OK {
		return nil, false
	}
	p := proc.MakeProcFromProto(res.ProcProto)

	pd.Lock()
	defer pd.Unlock()

	// Expects Lock to be held, since it does some resource accounting.
	var q string
	if p.GetType() == proc.T_LC {
		q = sp.PROCD_RUNQ_LC
	} else {
		q = sp.PROCD_RUNQ_BE
	}
	procPath := path.Join(sp.PROCD, pd.memfssrv.MyAddr(), q, p.GetPid().String())
	if ok := pd.claimProc(p, procPath); !ok {
		db.DFatalf("Failed to claim proc: %v", err)
	}
	linuxProc := pd.registerProcL(p, false)

	return linuxProc, true
}

func (pd *Procd) worker() {
	var p *LinuxProc
	var ok bool
	defer pd.workers.Done()
	for !pd.readDone() {
		db.DPrintf(db.PROCD, "Try to get proc.")
		if p, ok = pd.getProc(); !ok {
			db.DPrintf(db.PROCD, "No proc available.")
			continue
		}
		db.DPrintf(db.PROCD, "Got proc %v", p.SysPid)
		err := pd.fs.running(p)
		if err != nil {
			pd.perf.Done()
			db.DFatalf("Procd pub running error %v %T\n", err, err)
		}
		// Wake a new worker to take this worker's place.
		pd.workers.Add(1)
		go pd.runProc(p)
	}
}

func (pd *Procd) Work() {
	db.DPrintf(db.PROCD, "Work")
	pd.workers.Add(1)
	go func() {
		defer pd.workers.Done()
		pd.memfssrv.Serve()
		pd.memfssrv.Done()
	}()
	pd.workers.Add(1)
	go pd.worker()
	pd.workers.Wait()
}
