package procd

import (
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
	usync "ulambda/sync"
)

const (
	WORK_STEAL_TIMEOUT_MS = 100
)

type Procd struct {
	mu         sync.Mutex
	fs         *ProcdFs
	spawnChan  chan bool
	bin        string
	nid        uint64
	done       bool
	addr       string
	procs      map[string]bool
	coreBitmap []bool
	coresAvail proc.Tcore
	group      sync.WaitGroup
	perf       *perf.Perf
	procclnt   *procclnt.ProcClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func RunProcd(bin string, pprofPath string, utilPath string) {
	pd := &Procd{}
	pd.nid = 0
	pd.bin = bin

	pd.procs = make(map[string]bool)
	pd.coreBitmap = make([]bool, linuxsched.NCores)
	pd.coresAvail = proc.Tcore(linuxsched.NCores)
	pd.perf = perf.MakePerf()

	pd.makeFs()

	// Set up FilePriorityBags and create name/runq
	pd.spawnChan = make(chan bool)

	pd.addr = pd.MyAddr()

	pprof := pprofPath != ""
	if pprof {
		pd.perf.SetupPprof(pprofPath)
	}
	// Must set core affinity before starting CPU Util measurements
	pd.setCoreAffinity()
	util := utilPath != ""
	if util {
		pd.perf.SetupCPUUtil(perf.CPU_UTIL_HZ, utilPath)
	}

	pd.MemFs.GetStats().MonitorCPUUtil()

	pd.Work()
}

func (pd *Procd) makeProc(a *proc.Proc) *Proc {
	p := &Proc{}
	p.pd = pd
	p.init(a)
	return p
}

// Evict all procs running in this procd
func (pd *Procd) evictProcsL() {
	for pid, _ := range pd.procs {
		pd.procclnt.EvictProcd(pid)
	}
}

func (pd *Procd) Done() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.done = true
	pd.perf.Teardown()
	pd.evictProcsL()
}

func (pd *Procd) readDone() bool {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	return pd.done
}

// XXX Statsd information?
// Check if this procd instance is able to satisfy a job's constraints.
// Trivially true when not benchmarking.
func (pd *Procd) satisfiesConstraintsL(p *proc.Proc) bool {
	// If we have enough cores to run this job...
	if pd.coresAvail >= p.Ncore {
		return true
	}
	return false
}

// Update resource accounting information.
func (pd *Procd) decrementResourcesL(p *proc.Proc) {
	pd.coresAvail -= p.Ncore
}

// Update resource accounting information.
func (pd *Procd) incrementResources(p *proc.Proc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.incrementResourcesL(p)
}

func (pd *Procd) incrementResourcesL(p *proc.Proc) {
	pd.coresAvail += p.Ncore
}

// Tries to get a runnable proc using the functions passed in. Allows for code reuse across local & remote runqs.
func (pd *Procd) getRunnableProc(procdPath string, queueName string, readRunq readRunqFn, readProc readProcFn, claimProc claimProcFn) (*proc.Proc, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	fs, err := readRunq(procdPath, queueName)
	if err != nil {
		return nil, err
	}

	// Read through procs
	for _, f := range fs {
		p, err := readProc(procdPath, queueName, f.Name)
		// Proc may have been stolen
		if err != nil {
			db.DLPrintf("PROCD", "Error getting RunqProc: %v", err)
			continue
		}
		if pd.satisfiesConstraintsL(p) {
			// Proc may have been stolen
			if ok := claimProc(procdPath, queueName, p); !ok {
				continue
			}
			pd.decrementResourcesL(p)
			return p, nil
		}
	}
	return nil, nil
}

func (pd *Procd) getProc() (*proc.Proc, error) {
	// First try and get any LC procs, else get a BE proc.
	runqs := []string{np.PROCD_RUNQ_LC, np.PROCD_RUNQ_BE}
	for _, runq := range runqs {
		// First, try to read from the local procdfs
		p, err := pd.getRunnableProc("", runq, pd.fs.readRunq, pd.fs.readRunqProc, pd.fs.claimProc)
		if p != nil || err != nil {
			return p, err
		}

		// Try to steal from other procds
		_, err = pd.ProcessDir(np.PROCD, func(st *np.Stat) (bool, error) {
			// don't process self
			if strings.HasPrefix(st.Name, pd.MyAddr()) {
				return false, nil
			}
			p, err = pd.getRunnableProc(path.Join(np.PROCD, st.Name), runq, pd.readRemoteRunq, pd.readRemoteRunqProc, pd.claimRemoteProc)
			if err != nil {
				db.DLPrintf("PROCD", "Error getRunnableProc in Procd.getProc: %v", err)
				return false, nil
			}
			if p != nil {
				return true, nil
			}
			return false, nil
		})
		if p != nil || err != nil {
			return p, err
		}
	}
	return nil, nil
}

func (pd *Procd) allocCores(n proc.Tcore) []uint {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	cores := []uint{}
	for i := 0; i < len(pd.coreBitmap); i++ {
		// If lambda asks for 0 cores, run on any core
		if n == proc.C_DEF {
			cores = append(cores, uint(i))
		} else {
			if !pd.coreBitmap[i] {
				pd.coreBitmap[i] = true
				cores = append(cores, uint(i))
				n -= 1
				if n == 0 {
					break
				}
			}
		}
	}
	return cores
}

func (pd *Procd) freeCores(cores []uint) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	for _, i := range cores {
		pd.coreBitmap[i] = false
	}
}

func (pd *Procd) runProc(p *Proc) {
	// Register running proc
	pd.mu.Lock()
	pd.procs[p.Pid] = true
	pd.mu.Unlock()

	// Allocate dedicated cores for this lambda to run on.
	cores := pd.allocCores(p.attr.Ncore)

	// If this proc doesn't require cores, start another worker to take our place
	// so we can make progress.
	done := int32(0)
	if p.attr.Ncore == 0 {
		pd.group.Add(1)
		go pd.worker(&done)
	}

	// Run the proc.
	p.run(cores)

	// Kill the old worker so we don't have too many workers running
	atomic.StoreInt32(&done, 1)

	// Free resources and dedicated cores.
	pd.freeCores(cores)
	pd.incrementResources(p.attr)

	// Deregister running procs
	pd.mu.Lock()
	delete(pd.procs, p.Pid)
	pd.mu.Unlock()
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

func (pd *Procd) waitSpawnOrTimeout(ticker *time.Ticker) {
	// Wait until either there was a spawn, or the timeout expires.
	select {
	case _, ok := <-pd.spawnChan:
		// If channel closed, return
		if !ok {
			return
		}
	case <-ticker.C:
	}
}

// Worker runs one proc a time
func (pd *Procd) worker(done *int32) {
	defer pd.group.Done()
	ticker := time.NewTicker(WORK_STEAL_TIMEOUT_MS * time.Millisecond)
	for !pd.readDone() && (done == nil || atomic.LoadInt32(done) == 0) {
		p, err := pd.getProc()
		// If there were no runnable procs, wait and try again.
		if err == nil && p == nil {
			pd.waitSpawnOrTimeout(ticker)
			continue
		}
		if err != nil && (err == io.EOF || strings.Contains(err.Error(), "no mount")) {
			continue
		}
		if err != nil {
			if strings.Contains(err.Error(), "file not found "+usync.COND) {
				db.DLPrintf("PROCD", "cond file not found: %v", err)
				return
			}
			log.Fatalf("Procd GetProc error %v, %v\n", p, err)
		}
		localProc := pd.makeProc(p)
		err = pd.fs.running(localProc)
		if err != nil {
			log.Fatalf("Procd pub running error %v\n", err)
		}
		pd.runProc(localProc)
	}
}

func (pd *Procd) Work() {
	go func() {
		pd.Serve()
		pd.Done()
		pd.MemFs.Done()
	}()
	// XXX May need a certain number of workers for tests, but need
	// NWorkers = NCores for benchmarks
	// The +1 is needed so procs trying to spawn a new proc never deadlock if this
	// procd is full
	NWorkers := linuxsched.NCores + 1
	for i := uint(0); i < NWorkers; i++ {
		pd.group.Add(1)
		go pd.worker(nil)
	}
	pd.group.Wait()
}
