package procd

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/inode"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procbase"
	"ulambda/procinit"
	usync "ulambda/sync"
)

const (
	NO_OP_LAMBDA = "no-op-lambda"
)

type Procd struct {
	mu         sync.Mutex
	runq       *usync.FilePriorityBag
	bin        string
	nid        uint64
	root       fs.Dir
	done       bool
	addr       string
	procs      map[string]bool
	coreBitmap []bool
	coresAvail proc.Tcore
	group      sync.WaitGroup
	perf       *perf.Perf
	procclnt   *procbase.ProcBaseClnt
	*fslib.FsLib
	*fssrv.FsServer
}

func RunProcd(bin string, pid string, pprofPath string, utilPath string) {
	var err error

	pd := &Procd{}
	pd.nid = 0
	pd.bin = bin

	pd.procs = make(map[string]bool)
	pd.coreBitmap = make([]bool, linuxsched.NCores)
	pd.coresAvail = proc.Tcore(linuxsched.NCores)
	pd.perf = perf.MakePerf()

	pd.root, pd.FsServer, pd.FsLib, err = fslibsrv.MakeMemFs(named.PROCD, "procd")
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}
	pd.addr = pd.MyAddr()
	pd.procclnt = procbase.MakeProcBaseClnt(pd.FsLib)

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
	// Make some directories used by other services.
	os.Mkdir(namespace.NAMESPACE_DIR, 0777)
	// Set up FilePriorityBags
	pd.runq = usync.MakeFilePriorityBag(pd.FsLib, procbase.RUNQ)

	procdStartCond := usync.MakeCond(pd.FsLib, path.Join(named.BOOT, pid), nil, true)
	procdStartCond.Destroy()

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})
	pd.FsServer.GetStats().MonitorCPUUtil(pd.FsLib)

	pd.Work()
}

func (pd *Procd) spawn(a *proc.Proc) (*Proc, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	p := &Proc{}
	p.FsObj = inode.MakeInode("", np.DMDEVICE, pd.root)
	p.pd = pd
	p.init(a)
	err := dir.MkNod(fssrv.MkCtx(""), pd.root, p.Pid, p)
	if err != nil {
		return nil, err
	}
	return p, nil
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

func (pd *Procd) readDone() bool {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	return pd.done
}

// XXX Statsd information?
// Check if this procd instance is able to satisfy a job's constraints.
// Trivially true when not benchmarking.
func (pd *Procd) satisfiesConstraints(p *proc.Proc) bool {
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

func (pd *Procd) getProc() (*proc.Proc, error) {
	pPriority, pName, b, err := pd.runq.Get()
	if err != nil {
		return nil, err
	}

	p := proc.MakeEmptyProc()
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Couldn't unmarshal proc file in Procd.getProc: %v, %v", string(b), err)
		return nil, err
	}

	if pd.satisfiesConstraints(p) {
		pd.decrementResourcesL(p)
		return p, nil
	} else {
		err = pd.runq.Put(pPriority, pName, b)
		if err != nil {
			log.Fatalf("Error Put in Procd.getProc: %v", err)
		}
	}

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

func (pd *Procd) runAll(ps []*Proc) {
	// Register running procs
	pd.mu.Lock()
	for _, p := range ps {
		pd.procs[p.Pid] = true
	}
	pd.mu.Unlock()

	var wg sync.WaitGroup
	for _, p := range ps {
		wg.Add(1)
		go func(p *Proc) {
			defer wg.Done()

			// Allocate dedicated cores for this lambda to run on.
			cores := pd.allocCores(p.attr.Ncore)

			// Run the lambda.
			p.run(cores)

			// Free resources and dedicated cores.
			pd.incrementResources(p.attr)
			pd.freeCores(cores)

		}(p)
	}
	wg.Wait()

	// Deregister running procs
	pd.mu.Lock()
	for _, p := range ps {
		delete(pd.procs, p.Pid)
	}
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

// Worker runs one lambda at a time
func (pd *Procd) Worker(workerId uint) {
	go func() {
		pd.Serve()
		pd.Done()
	}()
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
		// XXX return err from spawn
		l, err := pd.spawn(p)
		if err != nil {
			log.Fatalf("Procd spawn error %v\n", err)
		}
		ls := []*Proc{l}
		//		ls = append(ls, consumerLs...)
		pd.runAll(ls)
	}
}

func (pd *Procd) Work() {
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
