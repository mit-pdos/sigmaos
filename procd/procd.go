package procd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	PROC_CACHE_SIZE = 500
)

type Procd struct {
	sync.Mutex
	*sync.Cond
	fs               *ProcdFs
	realm            string                   // realm id of this procd
	nToWake          int                      // Number of worker threads to wake. This is incremented on proc spawn and when this procd is granted more cores.
	wsQueues         map[string][]string      // Map containing queues of procs which may be available to steal. Periodically updated by one thread.
	done             bool                     // Finished running.
	kernelInitDone   bool                     // True if kernel init has finished (this procd has spawned ux & s3).
	kernelProcs      map[string]bool          // Map of kernel procs spawned on this procd.
	addr             string                   // Address of this procd.
	procClaimTime    time.Time                // Time used to rate-limit claiming of BE procs.
	netProcsClaimed  proc.Tcore               // Number of BE procs claimed in the last time interval.
	procsDownloading proc.Tcore               // Number of procs currently being downloaded.
	runningProcs     map[proc.Tpid]*LinuxProc // Map of currently running procs.
	coreBitmap       []Tcorestatus            // Bitmap of cores owned by this proc
	cpuMask          linuxsched.CPUMask       // Mask of CPUs available for this procd to use.
	coresOwned       proc.Tcore               // Current number of cores which this procd "owns", and can run procs on.
	coresAvail       proc.Tcore               // Current number of cores available to run procs on.
	memAvail         proc.Tmem                // Available memory for this procd and its procs to use.
	pcache           *ProcCache
	perf             *perf.Perf
	group            sync.WaitGroup
	procclnt         *procclnt.ProcClnt
	memfssrv         *memfssrv.MemFs
	*fslib.FsLib
}

func RunProcd(realm string, grantedCoresIv string, spawningSys bool) {
	pd := &Procd{}
	pd.pcache = MakeProcCache(PROC_CACHE_SIZE)
	pd.Cond = sync.NewCond(&pd.Mutex)
	pd.kernelProcs = make(map[string]bool)
	pd.kernelProcs["kernel/dbd"] = true
	// If we aren't spawning a full system on this procd, then kernel init is
	// done (this procd can start to accept procs).
	if !spawningSys {
		pd.kernelInitDone = true
	}
	pd.realm = realm
	pd.wsQueues = make(map[string][]string)
	pd.runningProcs = make(map[proc.Tpid]*LinuxProc)
	pd.coreBitmap = make([]Tcorestatus, linuxsched.NCores)
	pd.coresAvail = proc.Tcore(linuxsched.NCores)
	pd.coresOwned = proc.Tcore(linuxsched.NCores)
	pd.memAvail = getMemTotal()

	pd.makeFs()

	pd.addr = pd.memfssrv.MyAddr()

	pd.initCores(grantedCoresIv)

	perf, err := perf.MakePerf("PROCD")
	if err != nil {
		log.Printf("MakePerf err %v\n", err)
	} else {
		pd.perf = perf
		defer pd.perf.Done()
	}

	// Make a directory in which to put stealable procs.
	pd.MkDir(sp.PROCD_WS, 0777)
	pd.MkDir(path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_LC), 0777)
	pd.MkDir(path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_BE), 0777)
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
		if p.SysPid == 0 || p.attr.Type != proc.T_LC || p.attr.IsPrivilegedProc() {
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
	pd.runningProcs[p.attr.Pid] = p
}

func (pd *Procd) deleteProc(p *LinuxProc) {
	pd.Lock()
	defer pd.Unlock()
	delete(pd.runningProcs, p.attr.Pid)
}

func (pd *Procd) spawnProc(a *proc.Proc) {
	pd.wakeWorker()
}

func (pd *Procd) wakeWorker() {
	pd.Lock()
	defer pd.Unlock()

	pd.wakeWorkerL()
}

func (pd *Procd) wakeWorkerL() {
	pd.nToWake++
	pd.Signal()
	db.DPrintf(db.PROCD, "Wake worker cnt %v", pd.nToWake)
}

func (pd *Procd) wsQueuePop(runq string) (string, bool) {
	pd.Lock()
	defer pd.Unlock()

	var pid string
	var ok bool
	if len(pd.wsQueues[runq]) > 0 {
		// Pop a proc from the ws queue
		pid, pd.wsQueues[runq] = pd.wsQueues[runq][0], pd.wsQueues[runq][1:]
		ok = true
	}
	return pid, ok
}

func (pd *Procd) wsQueuePush(pid string, runq string) {
	pd.Lock()
	defer pd.Unlock()
	pd.wsQueues[runq] = append(pd.wsQueues[runq], pid)
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
	pd.Broadcast()
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
	pd.allocCoresL(linuxProc, p.Ncore)
	pd.allocMemL(p)
	// Register running proc in in-memory structures.
	pd.putProcL(linuxProc)
	return linuxProc
}

// Tries to claim a runnable proc if it fits on this procd.
func (pd *Procd) tryClaimProc(procPath string, isRemote bool) (*LinuxProc, error) {
	// XXX shouldn't I just lock after reading the proc?
	pd.Lock()
	defer pd.Unlock()

	if db.WillBePrinted(db.PROCD) {
		db.DPrintf(db.PROCD, "Try get runnable proc %v", path.Base(procPath))
	}
	p, err := pd.readRunqProc(procPath)
	// Proc may have been stolen
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "Error getting RunqProc: %v", err)
		return nil, err
	}
	// Don't steal remote kernel procs.
	if isRemote && p.IsPrivilegedProc() {
		return nil, fmt.Errorf("Try steal remote kernel proc")
	}
	// See if the proc fits on this procd. Also, make sure that we spawn all
	// kernel procs before any user procs.
	if pd.hasEnoughCores(p) && pd.hasEnoughMemL(p) && (pd.kernelInitDone || p.IsPrivilegedProc()) {
		// Proc may have been stolen
		if ok := pd.claimProc(p, procPath); !ok {
			return nil, nil
		}
		linuxProc := pd.registerProcL(p, isRemote)
		return linuxProc, nil
	} else {
		if db.WillBePrinted(db.PROCD) {
			db.DPrintf(db.PROCD, "RunqProc %v didn't satisfy constraints", procPath)
		}
	}
	return nil, nil
}

// Returns true if the proc was there, but there was a capacity issue.
func (pd *Procd) tryGetProc(procPath string, isRemote bool) (*LinuxProc, bool) {
	// We need to add "/" to follow the symlink for remote queues.
	if isRemote {
		procPath += "/"
	}
	newProc, err := pd.tryClaimProc(procPath, isRemote)
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "Error getting runnable proc (remote:%v): %v", isRemote, err)
		// Remove the symlink, as it must have already been claimed.
		if isRemote {
			pd.deleteWSSymlink(procPath, newProc, isRemote)
		}
	}
	// We claimed a proc successfully, so delete the work stealing symlink for
	// this proc.
	if newProc != nil {
		pd.deleteWSSymlink(procPath, newProc, isRemote)
	}
	// If newProc and err are both nil, then the reason we couldn't claim this
	// proc is that we don't have enough capacity. Note this to higher layers.
	return newProc, newProc == nil && err == nil
}

func (pd *Procd) getProc() (*LinuxProc, error) {
	var p *LinuxProc
	// First try and get any LC procs, else get a BE proc.
	localPath := path.Join(sp.PROCD, pd.memfssrv.MyAddr())
	// Claim order:
	// 1. local LC queue
	// 2. remote LC queue
	// 3. local BE queue
	// 4. remote BE queue
	runqs := []string{
		path.Join(localPath, sp.PROCD_RUNQ_LC),
		path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_LC),
		path.Join(localPath, sp.PROCD_RUNQ_BE),
		path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_BE),
	}
	for i, runq := range runqs {
		// If this is a BE queue, and we couldn't possibly claim a BE proc, skip
		// scanning the queue.
		if isBE := i > 1; isBE && !pd.canClaimBEProc() {
			continue
		}
		// Odd indices are remote queues.
		isRemote := i%2 == 1
		if isRemote {
			// Instead of having every worker thread bang on named to try to steal
			// procs, one procd thread (the Work Stealing Monitor) periodically scans
			// the work stealing queues and caches names of stealable procs in a
			// local slice. Worker threads iterate through this slice when trying to
			// steal procs.
			pd.Lock()
			// Find number of procs in this queue.
			n := len(pd.wsQueues[runq])
			pd.Unlock()
			// Iterate through (up to) n items in the queue, or until we've claimed a
			// proc.
			for j := 0; j < n && p == nil; j++ {
				// If the queue was empty, we're done scanning this queue. This may
				// occur before the loop naturally terminates because:
				//
				// 1. Other worker threads on this procd popped off the remaining queue
				// elements.
				// 2. The monitor thread updated the queue, and it now
				// contains fewer elements.
				pid, ok := pd.wsQueuePop(runq)
				if !ok {
					break
				}
				procPath := path.Join(runq, pid)
				// Try to get the proc.
				var noCapacity bool
				p, noCapacity = pd.tryGetProc(procPath, isRemote)
				// If the reason we couldn't claim this proc is that there wasn't
				// enough capacity, then add it back to the queue so we can try and
				// claim it later.
				if p == nil && noCapacity {
					//					pd.wsQueuePush(pid, runq)
				}
			}
			// If the proc was successfully claimed, we're done
			if p != nil {
				break
			}
		} else {
			_, err := pd.ProcessDir(runq, func(st *sp.Stat) (bool, error) {
				procPath := path.Join(runq, st.Name)
				p, _ = pd.tryGetProc(procPath, isRemote)
				// If a proc was not claimed, keep processing.
				if p == nil {
					return false, nil
				}
				return true, nil
			})
			if err != nil {
				return nil, err
			}
			// If the proc was successfully claimed, we're done
			if p != nil {
				break
			}
		}
	}
	return p, nil
}

func (pd *Procd) runProc(p *LinuxProc) {
	if !p.attr.IsPrivilegedProc() {
		// Download the bin from s3, if it isn't already cached locally.
		pd.downloadProcBin(p.attr.Program)
	}

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
	pd.memfssrv.GetStats().UpdateCores()
}

// Wait for a new proc to be spawned at this procd, or for a stealing
// opportunity to present itself.
func (pd *Procd) waitSpawnOrSteal() {
	pd.Lock()
	defer pd.Unlock()

	for !pd.done {
		// If there is an LC proc available to work-steal, and this procd has cores
		// to spare, release the worker thread.
		db.DPrintf(db.PROCD, "Worker woke, check for stealable LC procs.")
		if len(pd.wsQueues[WS_LC_QUEUE_PATH]) > 0 && pd.coresAvail > 0 {
			db.DPrintf(db.PROCD, "done waiting, an LC proc can be stolen")
			return
		}
		// If there is a BE proc available to work-steal, and this procd can run
		// another one, release the worker thread.
		db.DPrintf(db.PROCD, "Worker woke, check for stealable BE procs.")
		if len(pd.wsQueues[WS_BE_QUEUE_PATH]) > 0 {
			_, _, ok := pd.canClaimBEProcL()
			// XXX a bit hacky... should do something more principled.
			if ok && pd.memAvail > 500 {
				return
			}
		}
		// Only release nToWake worker threads.
		if pd.nToWake > 0 {
			pd.nToWake--
			db.DPrintf(db.PROCD, "done waiting, worker woken. %v left to wake", pd.nToWake)
			return
		}
		db.DPrintf(db.PROCD, "Worker wait %v %v %v", pd.nToWake, len(pd.wsQueues[sp.PROCD_RUNQ_LC]), len(pd.wsQueues[sp.PROCD_RUNQ_BE]))
		pd.Wait()
	}
}

// Worker runs one proc a time. If the proc it runs has Ncore == 0, then
// another worker is spawned to take this one's place. This worker will then
// exit once it finishes running the proc.
func (pd *Procd) worker() {
	defer pd.group.Done()
	for !pd.readDone() {
		db.DPrintf(db.PROCD, "Try to get proc.")
		p, error := pd.getProc()
		// If there were no runnable procs, wait and try again.
		if error == nil && p == nil {
			db.DPrintf(db.PROCD, "No procs found, waiting.")
			pd.waitSpawnOrSteal()
			continue
		}
		if error != nil && (errors.Is(error, io.EOF) || serr.IsErrUnreachable(error)) {
			continue
		}
		if error != nil {
			if serr.IsErrNotfound(error) {
				db.DPrintf(db.PROCD_ERR, "cond file not found: %v", error)
				return
			}
			pd.perf.Done()
			db.DFatalf("Procd GetProc error %v, %v\n", p, error)
		}
		db.DPrintf(db.PROCD, "Got proc %v", p.SysPid)
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
			// Wake a new worker to take this worker's place.
			pd.wakeWorker()
			pd.group.Add(1)
			go pd.worker()
		}
		pd.runProc(p)
		// Wake a worker, since we may be able to run something else now.
		pd.wakeWorker()
		if replaced {
			return
		}
	}
}

func (pd *Procd) Work() {
	db.DPrintf(db.PROCD, "Work")
	pd.group.Add(1)
	go func() {
		defer pd.group.Done()
		pd.memfssrv.Serve()
		pd.memfssrv.Done()
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
