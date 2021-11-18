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
	"ulambda/fssrv"
	"ulambda/inode"
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
	NO_OP_LAMBDA = "no-op-lambda"
)

const (
	RUNQLC_PRIORITY = "0"
	RUNQ_PRIORITY   = "1"
)

type ProcdFs struct {
	root    fs.Dir
	running fs.Dir
	runq    fs.Dir
	ctlFile fs.FsObj
}

type Procd struct {
	mu         sync.Mutex
	fs         *ProcdFs
	localRunq  chan *proc.Proc
	globalRunq *usync.FilePriorityBag
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
	pd.localRunq = make(chan *proc.Proc)
	pd.globalRunq = usync.MakeFilePriorityBag(pd.FsLib, procclnt.RUNQ)

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

func (pd *Procd) spawn(a *proc.Proc) (*Proc, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	p := &Proc{}
	p.FsObj = inode.MakeInode("", np.DMDEVICE, pd.fs.running)
	p.pd = pd
	p.init(a)
	err := dir.MkNod(fssrv.MkCtx(""), pd.fs.running, p.Pid, p)
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
	close(pd.localRunq)
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

func getPriority(p *proc.Proc) string {
	var procPriority string
	switch p.Type {
	case proc.T_DEF:
		procPriority = RUNQ_PRIORITY
	case proc.T_LC:
		procPriority = RUNQLC_PRIORITY
	case proc.T_BE:
		procPriority = RUNQ_PRIORITY
	default:
		log.Fatalf("Error in CtlFile.Write: Unknown proc type %v", p.Type)
	}
	return procPriority
}

func (pd *Procd) getProc() (*proc.Proc, error) {
	var p *proc.Proc
	var b []byte
	var ok bool
	var err error
	p, ok = <-pd.localRunq
	if ok { // If we had a proc waiting locally
		b, err = json.Marshal(p)
		if err != nil {
			log.Fatalf("Couldn't unmarshal proc file in Procd.getProc: %v, %v", string(b), err)
			return nil, err
		}
	} else {
		// XXX for now, if nothing is available locally, spin.
		return nil, nil
		// XXX We need a conditional get or something here, else this may block too long
		//		_, _, b, err := pd.globalRunq.Get()
		//		if err != nil {
		//			return nil, err
		//		}
		//
		//		p = proc.MakeEmptyProc()
		//		err = json.Unmarshal(b, p)
		//		if err != nil {
		//			log.Fatalf("Couldn't unmarshal proc file in Procd.getProc: %v, %v", string(b), err)
		//			return nil, err
		//		}
	}

	if pd.satisfiesConstraints(p) {
		pd.decrementResourcesL(p)
		return p, nil
	} else {
		// XXX This seems a bit inefficient
		// If it isn't runnable, we put it back on the globalRunq
		err = pd.globalRunq.Put(getPriority(p), p.Pid, b)
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
