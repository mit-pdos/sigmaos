package procd

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/stats"
	usync "ulambda/sync"
)

const (
	PROCD_ROOT   = "name/procds"
	NO_OP_LAMBDA = "no-op-lambda"
)

type Procd struct {
	//	mu deadlock.Mutex
	mu         sync.Mutex
	runq       *usync.FileBag
	load       int // XXX bogus
	bin        string
	nid        uint64
	root       *Dir
	done       bool
	ip         string
	ls         map[string]*Lambda
	coreBitmap []bool
	coresAvail proc.Tcore
	srv        *npsrv.NpServer
	group      sync.WaitGroup
	perf       *perf.Perf
	st         *npo.SessionTable
	*fslib.FsLib
	*proc.ProcCtl
}

func MakeProcd(bin string, pprofPath string, utilPath string) *Procd {
	pd := &Procd{}
	pd.load = 0
	pd.nid = 0
	pd.bin = bin
	db.Name("procd")
	pd.root = pd.makeDir([]string{}, np.DMDIR, nil)
	pd.root.time = time.Now().Unix()
	pd.st = npo.MakeSessionTable()
	pd.ls = map[string]*Lambda{}
	pd.coreBitmap = make([]bool, linuxsched.NCores)
	pd.coresAvail = proc.Tcore(linuxsched.NCores)
	pd.perf = perf.MakePerf()
	ip, err := fsclnt.LocalIP()
	pd.ip = ip
	if err != nil {
		log.Fatalf("LocalIP %v\n", err)
	}
	pd.srv = npsrv.MakeNpServer(pd, pd.ip+":0")
	fsl := fslib.MakeFsLib("procd")
	fsl.Mkdir(PROCD_ROOT, 0777)
	pd.FsLib = fsl
	pd.ProcCtl = proc.MakeProcCtl(fsl)
	err = fsl.PostServiceUnion(pd.srv.MyAddr(), fslib.PROCD_ROOT, pd.srv.MyAddr())
	if err != nil {
		log.Fatalf("procd PostServiceUnion failed %v %v\n", pd.srv.MyAddr(), err)
	}
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
	fsl.Mkdir(proc.PROC_COND, 0777)
	fsl.Mkdir(fslib.LOCKS, 0777)
	fsl.Mkdir(fslib.TMP, 0777)
	// Set up FileBags
	pd.runq = usync.MakeFileBag(fsl, proc.RUNQ)
	return pd
}

func (pd *Procd) spawn(p *proc.Proc) (*Lambda, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	l := &Lambda{}
	l.pd = pd
	l.init(p)
	pd.ls[l.Pid] = l
	return l, nil
}

func (pd *Procd) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(pd, conn)
}

func (pd *Procd) Done() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.done = true
	pd.perf.Teardown()
	pd.runq.Destroy()
}

func (pd *Procd) WatchTable() *npo.WatchTable {
	return nil
}

func (pd *Procd) ConnTable() *npo.ConnTable {
	return nil
}

func (pd *Procd) RegisterSession(sess np.Tsession) {
	pd.st.RegisterSession(sess)
}

func (pd *Procd) SessionTable() *npo.SessionTable {
	return pd.st
}

func (pd *Procd) Stats() *stats.Stats {
	return nil
}

func (pd *Procd) readDone() bool {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	return pd.done
}

func (pd *Procd) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return pd.root, nil
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
	pName, b, err := pd.runq.Get()
	if err != nil {
		return nil, err
	}

	p := &proc.Proc{}
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Couldn't unmarshal proc file in Procd.getProc: %v, %v", string(b), err)
		return nil, err
	}

	if pd.satisfiesConstraints(p) {
		pd.decrementResourcesL(p)
		return p, nil
	} else {
		err = pd.runq.Put(pName, b)
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

func (pd *Procd) runAll(ls []*Lambda) {
	var wg sync.WaitGroup
	for _, l := range ls {
		wg.Add(1)
		go func(l *Lambda) {
			defer wg.Done()

			// Allocate dedicated cores for this lambda to run on.
			cores := pd.allocCores(l.attr.Ncore)

			// Run the lambda.
			l.run(cores)

			// Free resources and dedicated cores.
			pd.incrementResources(l.attr)
			pd.freeCores(cores)

		}(l)
	}
	wg.Wait()
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
			log.Fatalf("Procd GetLambda error %v, %v\n", p, err)
		}
		// XXX return err from spawn
		l, err := pd.spawn(p)
		if err != nil {
			log.Fatalf("Procd spawn error %v\n", err)
		}
		ls := []*Lambda{l}
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
