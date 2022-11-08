package procd

import (
	"errors"
	"io"
	"os"
	"path"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/fslibsrv"
	"sigmaos/linuxsched"
	"sigmaos/namespace"
	np "sigmaos/ninep"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

type Procd struct {
	mu               sync.Mutex
	fs               *ProcdFs
	realmbin         string    // realm path from which to pull/run bins.
	spawnChan        chan bool // Indicates a proc has been spawned on this procd.
	stealChan        chan bool // Indicates there is work to be stolen.
	done             bool
	addr             string
	procClaimTime    time.Time  // Time used to rate-limit claiming of BE procs.
	netProcsClaimed  proc.Tcore // Number of BE procs claimed in the last time interval.
	procsDownloading proc.Tcore // Number of procs currently being downloaded.
	runningProcs     map[proc.Tpid]*LinuxProc
	coreBitmap       []Tcorestatus
	cpuMask          linuxsched.CPUMask
	coresOwned       proc.Tcore // Current number of cores which this procd "owns", and can run procs on.
	coresAvail       proc.Tcore // Current number of cores available to run procs on.
	memAvail         proc.Tmem
	perf             *perf.Perf
	group            sync.WaitGroup
	procclnt         *procclnt.ProcClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func RunProcd(realmbin string, grantedCoresIv string) {
	pd := &Procd{}
	pd.realmbin = realmbin

	pd.runningProcs = make(map[proc.Tpid]*LinuxProc)
	pd.coreBitmap = make([]Tcorestatus, linuxsched.NCores)
	pd.coresAvail = proc.Tcore(linuxsched.NCores)
	pd.coresOwned = proc.Tcore(linuxsched.NCores)
	pd.memAvail = getMemTotal()

	pd.makeFs()

	pd.addr = pd.MyAddr()

	pd.spawnChan = make(chan bool)
	pd.stealChan = make(chan bool)

	pd.initCores(grantedCoresIv)

	pd.perf = perf.MakePerf("PROCD")
	defer pd.perf.Done()

	// Make namespace isolation dir.
	os.MkdirAll(namespace.NAMESPACE_DIR, 0777)

	// Make a directory in which to put stealable procs.
	pd.MkDir(np.PROCD_WS, 0777)
	pd.MkDir(path.Join(np.PROCD_WS, np.PROCD_RUNQ_LC), 0777)
	pd.MkDir(path.Join(np.PROCD_WS, np.PROCD_RUNQ_BE), 0777)
	pd.MemFs.GetStats().DisablePathCnts()
	pd.MemFs.GetStats().MonitorCPUUtil(pd.getLCProcUtil)

	pd.Work()
}

func (pd *Procd) getLCProcUtil() float64 {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	var total float64 = 0.0
	for _, p := range pd.runningProcs {
		// If proc has not been initialized, or it isn't LC, move on
		if p.SysPid == 0 || p.attr.Type != proc.T_LC {
			continue
		}
		total += p.getUtilL()
	}
	return total
}

// Caller holds lock.
func (pd *Procd) putProcL(p *LinuxProc) {
	pd.runningProcs[p.attr.Pid] = p
}

func (pd *Procd) deleteProc(p *LinuxProc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	delete(pd.runningProcs, p.attr.Pid)
}

func (pd *Procd) spawnProc(a *proc.Proc) {
	pd.spawnChan <- true
}

// Evict all procs running in this procd
func (pd *Procd) evictProcsL(procs map[proc.Tpid]*LinuxProc) {
	for pid, _ := range procs {
		pd.procclnt.EvictProcd(pd.addr, pid)
	}
}

func (pd *Procd) Done() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.done = true
	pd.perf.Done()
	pd.evictProcsL(pd.runningProcs)
	close(pd.spawnChan)
}

func (pd *Procd) readDone() bool {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	return pd.done
}

func (pd *Procd) registerProcL(p *proc.Proc) *LinuxProc {
	// Make a Linux Proc which corresponds to this proc.
	linuxProc := makeLinuxProc(pd, p)
	// Allocate dedicated cores for this proc to run on, if it requires them.
	// Core allocation & resource accounting has to happen at this point, while
	// we still hold the lock we used to claim the proc, since this procd may be
	// resized at any time. When the resize happens, we *must* have already
	// assigned cores to this proc & registered it in the procd in-mem data
	// structures so that the proc's core allocations will be adjusted during the
	// resize.
	pd.allocCoresL(linuxProc, p.Ncore)
	pd.allocMemL(p)
	// Register running proc in in-memory structures.
	pd.putProcL(linuxProc)
	return linuxProc
}

// Tries to get a runnable proc if it fits on this procd.
func (pd *Procd) tryGetRunnableProc(procPath string) (*LinuxProc, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	db.DPrintf("PROCD", "Try get runnable proc %v", path.Base(procPath))
	p, err := pd.readRunqProc(procPath)
	// Proc may have been stolen
	if err != nil {
		db.DPrintf("PROCD_ERR", "Error getting RunqProc: %v", err)
		return nil, err
	}
	// See if the proc fits on this procd.
	if pd.hasEnoughCores(p) && pd.hasEnoughMemL(p) {
		// Proc may have been stolen
		if ok := pd.claimProc(p, procPath); !ok {
			return nil, nil
		}
		linuxProc := pd.registerProcL(p)
		return linuxProc, nil
	} else {
		db.DPrintf("PROCD", "RunqProc %v didn't satisfy constraints", procPath)
	}
	return nil, nil
}

func (pd *Procd) getProc() (*LinuxProc, error) {
	var p *LinuxProc
	// First try and get any LC procs, else get a BE proc.
	localPath := path.Join(np.PROCD, pd.MyAddr())
	// Claim order:
	// 1. local LC queue
	// 2. remote LC queue
	// 3. local BE queue
	// 4. remote BE queue
	runqs := []string{
		path.Join(localPath, np.PROCD_RUNQ_LC),
		path.Join(np.PROCD_WS, np.PROCD_RUNQ_LC),
		path.Join(localPath, np.PROCD_RUNQ_BE),
		path.Join(np.PROCD_WS, np.PROCD_RUNQ_BE),
	}
	for i, runq := range runqs {
		// Odd indices are remote queues.
		isRemote := i%2 == 1
		_, err := pd.ProcessDir(runq, func(st *np.Stat) (bool, error) {
			procPath := path.Join(runq, st.Name)
			// We need to add "/" to follow the symlink for remote queues.
			if isRemote {
				procPath += "/"
			}
			newProc, err := pd.tryGetRunnableProc(procPath)
			if err != nil {
				db.DPrintf("PROCD_ERR", "Error getting runnable proc: %v", err)
				// Remove the symlink, as it must have already been claimed.
				if isRemote {
					pd.Remove(procPath)
				}
				return false, nil
			}
			// We claimed a proc successfully, so stop.
			if newProc != nil {
				p = newProc
				// Delete the work stealing symlink for this proc.
				pd.deleteWSSymlink(st, procPath, p, isRemote)
				return true, nil
			}
			// Couldn't claim a proc, so keep looking.
			return false, nil
		})
		if err != nil {
			return nil, err
		}
		if p != nil {
			return p, nil
		}
	}
	return p, nil
}

func (pd *Procd) runProc(p *LinuxProc) {
	// Download the bin from s3, if it isn't already cached locally.
	pd.downloadProcBin(p.attr.Program)

	// Run the proc.
	p.run()

	// Free any dedicated cores.
	pd.freeCores(p)
	pd.freeMem(p.attr)

	// Deregister running procs
	pd.deleteProc(p)
}

// Set the core affinity for procd, according to what cores it owns. Caller
// holds lock.
func (pd *Procd) setCoreAffinityL() {
	for i := uint(0); i < linuxsched.NCores; i++ {
		// Clear all cores from the CPU mask.
		pd.cpuMask.Clear(i)
		// If we own this core, set it in the CPU mask.
		if pd.coreBitmap[i] != CORE_BLOCKED {
			pd.cpuMask.Set(i)
		}
	}
	linuxsched.SchedSetAffinityAllTasks(os.Getpid(), &pd.cpuMask)
	// Update the set of cores whose CPU utilization we're monitoring.
	pd.MemFs.GetStats().UpdateCores()
}

// Wait for a new proc to be spawned at this procd, or for a stealing
// opportunity to present itself.
func (pd *Procd) waitSpawnOrSteal() {
	select {
	case _, _ = <-pd.spawnChan:
		db.DPrintf("PROCD", "done waiting, a proc was spawned")
	case _, _ = <-pd.stealChan:
		db.DPrintf("PROCD", "done waiting, a proc can be stolen")
	}
}

// Worker runs one proc a time. If the proc it runs has Ncore == 0, then
// another worker is spawned to take this one's place. This worker will then
// exit once it finishes runing the proc.
func (pd *Procd) worker() {
	defer pd.group.Done()
	for !pd.readDone() {
		db.DPrintf("PROCD", "Try to get proc.")
		p, error := pd.getProc()
		// If there were no runnable procs, wait and try again.
		if error == nil && p == nil {
			db.DPrintf("PROCD", "No procs found, waiting.")
			pd.waitSpawnOrSteal()
			continue
		}
		if error != nil && (errors.Is(error, io.EOF) || np.IsErrUnreachable(error)) {
			continue
		}
		if error != nil {
			if np.IsErrNotfound(error) {
				db.DPrintf("PROCD_ERR", "cond file not found: %v", error)
				return
			}
			pd.perf.Done()
			db.DFatalf("Procd GetProc error %v, %v\n", p, error)
		}
		db.DPrintf("PROCD", "Got proc %v", p)
		err := pd.fs.running(p)
		if err != nil {
			pd.perf.Done()
			db.DFatalf("Procd pub running error %v %T\n", err, err)
		}
		// If this proc doesn't require cores, start another worker to take our
		// place so user apps don't deadlock.
		replaced := false
		if p.attr.Ncore == 0 {
			replaced = true
			pd.group.Add(1)
			go pd.worker()
		}
		pd.runProc(p)
		if replaced {
			return
		}
	}
}

func (pd *Procd) Work() {
	db.DPrintf("PROCD", "Work")
	pd.group.Add(1)
	go func() {
		defer pd.group.Done()
		pd.Serve()
		pd.Done()
		pd.MemFs.Done()
	}()
	go pd.offerStealableProcs()
	pd.startWorkStealingMonitors()
	// The +1 is needed so procs trying to spawn a new proc never deadlock if this
	// procd is full
	NWorkers := linuxsched.NCores + 1
	for i := uint(0); i < NWorkers; i++ {
		pd.group.Add(1)
		go pd.worker()
	}
	pd.group.Wait()
}
