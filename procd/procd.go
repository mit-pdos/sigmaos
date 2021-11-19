package procd

import (
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/fssrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
	usync "ulambda/sync"
)

const (
	WORK_STEAL_TIMEOUT_MS = 10
)

type readRunqFn func() ([]*np.Stat, error)
type readProcFn func(pid string) (*proc.Proc, error)
type claimProcFn func(p *proc.Proc) bool

type Procd struct {
	mu         deadlock.Mutex
	fs         *ProcdFs
	procEvent  chan bool
	bin        string
	nid        uint64
	done       bool
	addr       string
	procs      map[string]bool
	coreBitmap []bool
	coresAvail proc.Tcore
	group      sync.WaitGroup
	perf       *perf.Perf
	procclnt   proc.ProcClnt
	*fslib.FsLib
	*fssrv.FsServer
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
	pd.procEvent = make(chan bool)

	pd.addr = pd.MyAddr()
	pd.procclnt = procclnt.MakeProcClnt(pd.FsLib)

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
	// Make some local directories.
	os.Mkdir(namespace.NAMESPACE_DIR, 0777)

	procdStartCond := usync.MakeCond(pd.FsLib, path.Join(named.BOOT, proc.GetPid()), nil, true)
	procdStartCond.Destroy()

	pd.FsServer.GetStats().MonitorCPUUtil(pd.FsLib)

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
		pd.procclnt.Evict(pid)
	}
}

func (pd *Procd) Done() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.done = true
	pd.perf.Teardown()
	pd.evictProcsL()
	pd.Exit()
}

func (pd *Procd) notifyProcEvent() {
	pd.procEvent <- true
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
	// Constraints are not checked when testing, as some tests require more cores
	// than we may have on our test machine.
	if !pd.perf.RunningBenchmark() {
		return true
	}
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
func (pd *Procd) getRunnableProc(readRunq readRunqFn, readProc readProcFn, claimProc claimProcFn) (*proc.Proc, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	fs, err := readRunq()
	if err != nil {
		log.Fatalf("Error readRunq in Procd.getRunnableProc: %v", err)
		return nil, err
	}

	// Read through procs
	for _, f := range fs {
		p, err := readProc(f.Name)
		// Proc may have been stolen
		if err != nil {
			log.Printf("Error getting RunqProc: %v", err)
			continue
		}
		if pd.satisfiesConstraintsL(p) {
			// Proc may have been stolen
			if ok := claimProc(p); !ok {
				continue
			}
			pd.decrementResourcesL(p)
			return p, nil
		}
	}
	return nil, nil
}

func (pd *Procd) getProc() (*proc.Proc, error) {
	ticker := time.NewTicker(WORK_STEAL_TIMEOUT_MS * time.Millisecond)

	// Wait until either there was a spawn or exit, or the timeout expires.
	select {
	case _, ok := <-pd.procEvent:
		// If channel closed, return
		if !ok {
			return nil, nil
		}
	case <-ticker.C:
	}

	// First, try to read from the local procdfs
	p, err := pd.getRunnableProc(pd.fs.readRunq, pd.fs.readRunqProc, pd.fs.claimProc)
	if p != nil || err != nil {
		return p, err
	}

	// TODO: Otherwise, try to steal from other procds.

	return nil, nil
}

func (pd *Procd) allocCores(n proc.Tcore) []uint {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	cores := []uint{}
	for i := 0; i < len(pd.coreBitmap); i++ {
		// If not running a benchmark or lambda asks for 0 cores, run on any core
		if !pd.perf.RunningBenchmark() || n == proc.C_DEF {
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

	// Run the proc.
	p.run(cores)

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
	// XXX For my current benchmarking setup, core 0 is reserved for ZK.
	if pd.perf.RunningBenchmark() {
		m.Clear(0)
		m.Clear(1)
	}
	linuxsched.SchedSetAffinityAllTasks(os.Getpid(), m)
}

// Worker runs one proc a time
func (pd *Procd) Worker(workerId uint) {
	defer pd.group.Done()
	for !pd.readDone() {
		p, err := pd.getProc()
		// If get failed, try again.
		if err == nil && p == nil {
			continue
		}
		if err != nil && (err == io.EOF || strings.Contains(err.Error(), "unknown mount")) {
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
		err = pd.fs.pubRunning(localProc)
		if err != nil {
			log.Fatalf("Procd pubRunning error %v\n", err)
		}
		pd.runProc(localProc)
	}
}

func (pd *Procd) Work() {
	go func() {
		pd.Serve()
		pd.Done()
	}()
	var NWorkers uint
	// XXX May need a certain number of workers for tests, but need
	// NWorkers = NCores for benchmarks
	if !pd.perf.RunningBenchmark() && linuxsched.NCores < 20 {
		NWorkers = 20
	} else {
		NWorkers = linuxsched.NCores
		if pd.perf.RunningBenchmark() {
			NWorkers -= 1
		}
	}
	for i := uint(0); i < NWorkers; i++ {
		pd.group.Add(1)
		go pd.Worker(i)
	}
	pd.group.Wait()
}
