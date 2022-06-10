package procd

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
)

type Procd struct {
	mu           sync.Mutex
	fs           *ProcdFs
	realmbin     string    // realm path from which to pull/run bins.
	spawnChan    chan bool // Indicates a proc has been spawned on this procd.
	stealChan    chan bool // Indicates there is work to be stolen.
	done         bool
	addr         string
	runningProcs map[proc.Tpid]*LinuxProc
	coreBitmap   []Tcorestatus
	coresAvail   proc.Tcore
	perf         *perf.Perf
	group        sync.WaitGroup
	procclnt     *procclnt.ProcClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func RunProcd(realmbin string) {
	pd := &Procd{}
	pd.realmbin = realmbin

	pd.runningProcs = make(map[proc.Tpid]*LinuxProc)
	pd.coreBitmap = make([]Tcorestatus, linuxsched.NCores)
	pd.coresAvail = proc.Tcore(linuxsched.NCores)

	// Must set core affinity before starting CPU Util measurements
	pd.setCoreAffinity()

	pd.perf = perf.MakePerf("PROCD")
	defer pd.perf.Done()

	pd.makeFs()

	// Set up FilePriorityBags and create name/runq
	pd.spawnChan = make(chan bool)
	pd.stealChan = make(chan bool)

	pd.addr = pd.MyAddr()

	pd.MemFs.GetStats().MonitorCPUUtil()

	// Make namespace isolation dir.
	os.MkdirAll(namespace.NAMESPACE_DIR, 0777)

	// Make a directory in which to put stealable procs.
	pd.MkDir(np.PROCD_WS, 0777)

	pd.Work()
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
func (pd *Procd) evictProcsL() {
	for pid, _ := range pd.runningProcs {
		pd.procclnt.EvictProcd(pd.addr, pid)
	}
}

func (pd *Procd) Done() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.done = true
	pd.perf.Done()
	pd.evictProcsL()
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
	pd.allocCoresL(linuxProc)
	// Register running proc in in-memory structures.
	pd.putProcL(linuxProc)
	return linuxProc
}

// Tries to get a runnable proc if it fits on this procd.
func (pd *Procd) tryGetRunnableProc(procPath string) (*LinuxProc, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	p, err := pd.readRunqProc(procPath)
	// Proc may have been stolen
	if err != nil {
		db.DPrintf("PROCD_ERR", "Error getting RunqProc: %v", err)
		return nil, err
	}
	// See if the proc fits on this procd.
	if pd.hasEnoughCores(p) {
		// Proc may have been stolen
		if ok := pd.claimProc(procPath); !ok {
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
	runqs := []string{np.PROCD_RUNQ_LC, np.PROCD_RUNQ_BE}
	// Try local procd first.
	for _, runq := range runqs {
		runqPath := path.Join(np.PROCD, pd.MyAddr(), runq)
		_, err := pd.ProcessDir(runqPath, func(st *np.Stat) (bool, error) {
			newProc, err := pd.tryGetRunnableProc(path.Join(runqPath, st.Name))
			if err != nil {
				db.DPrintf("PROCD_ERR", "Error getting runnable proc: %v", err)
				return false, nil
			}
			// We claimed a proc successfully, so stop.
			if newProc != nil {
				p = newProc
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
	// Try to steal from other procds.
	_, err := pd.ProcessDir(np.PROCD_WS, func(st *np.Stat) (bool, error) {
		procPath := path.Join(np.PROCD_WS, st.Name)
		newProc, err := pd.tryGetRunnableProc(procPath + "/")
		if err != nil {
			db.DPrintf("PROCD_ERR", "Error readRunqProc in Procd.getProc: %v", err)
			// Remove the symlink, as it must have already been claimed.
			pd.Remove(procPath)
			return false, nil
		}
		if newProc != nil {
			db.DPrintf("PROCD", "Stole proc: %v", newProc)
			p = newProc
			// Remove the ws symlink.
			if err := pd.Remove(procPath); err != nil {
				db.DPrintf("PROCD_ERR", "Error Remove symlink after claim: %v", err)
			}
			return true, nil
		}
		return false, nil
	})
	return p, err
}

func (pd *Procd) runProc(p *LinuxProc) {
	// Download the bin from s3, if it isn't already cached locally.
	pd.downloadProcBin(p.attr.Program)

	// Run the proc.
	p.run()

	// Free any dedicated cores.
	pd.freeCores(p)
	pd.incrementCores(p)

	// Deregister running procs
	pd.deleteProc(p)
}

func (pd *Procd) setCoreAffinity() {
	// XXX Currently, we just set the affinity for all available cores since Linux
	// seems to do a decent job of avoiding moving things around too much.
	m := &linuxsched.CPUMask{}
	for i := uint(0); i < linuxsched.NCores; i++ {
		m.Set(i)
	}
	linuxsched.SchedSetAffinityAllTasks(os.Getpid(), m)
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
		if error != nil && (errors.Is(error, io.EOF) ||
			(np.IsErrUnreachable(error) && strings.Contains(np.ErrPath(error), "no mount"))) {
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
	pd.group.Add(1)
	go func() {
		defer pd.group.Done()
		pd.Serve()
		pd.Done()
		pd.MemFs.Done()
	}()
	go pd.workStealingMonitor()
	go pd.offerStealableProcs()
	// The +1 is needed so procs trying to spawn a new proc never deadlock if this
	// procd is full
	NWorkers := linuxsched.NCores + 1
	for i := uint(0); i < NWorkers; i++ {
		pd.group.Add(1)
		go pd.worker()
	}
	pd.group.Wait()
}
