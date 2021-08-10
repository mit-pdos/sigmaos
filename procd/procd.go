package procd

import (
	//	"github.com/sasha-s/go-deadlock"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

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
)

const (
	NO_OP_LAMBDA = "no-op-lambda"
)

type Procd struct {
	//	mu deadlock.Mutex
	mu         sync.Mutex
	cond       *sync.Cond
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
	pd.cond = sync.NewCond(&pd.mu)
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
	fsl.Mkdir(fslib.PROCD_ROOT, 0777)
	pd.FsLib = fsl
	pd.ProcCtl = proc.MakeProcCtl(fsl, "procd-"+ip)
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
	// Try to make scheduling directories if they don't already exist
	fsl.Mkdir(proc.RUNQ, 0777)
	fsl.Mkdir(proc.RUNQLC, 0777)
	fsl.Mkdir(proc.WAITQ, 0777)
	fsl.Mkdir(proc.CLAIMED, 0777)
	fsl.Mkdir(proc.CLAIMED_EPH, 0777)
	fsl.Mkdir(proc.SPAWNED, 0777)
	fsl.Mkdir(fslib.LOCKS, 0777)
	fsl.Mkdir(fslib.TMP, 0777)
	return pd
}

func (pd *Procd) spawn(a []byte) (*Lambda, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	l := &Lambda{}
	l.pd = pd
	err := l.init(a)
	if err != nil {
		return nil, err
	}
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
	pd.SignalNewJob()
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

func (pd *Procd) getRun(runq string) ([]byte, error) {
	jobs, err := pd.ReadRunQ(runq)
	if err != nil {
		return []byte{}, err
	}
	for _, j := range jobs {
		pd.mu.Lock()
		// Read a job
		b, err := pd.ReadJob(runq, j.Name)
		// If job has already been claimed, move on
		if err != nil {
			pd.mu.Unlock()
			continue
		}
		// Unmarshal it
		p := unmarshalJob(b)
		// See if we can run it, and if so, try to claim it
		claimed := false
		if pd.satisfiesConstraints(p) {
			b, claimed = pd.ClaimRunQJob(runq, j.Name)
		}
		if claimed {
			pd.decrementResourcesL(p)
			pd.mu.Unlock()
			return b, nil
		}
		pd.mu.Unlock()
	}
	return []byte{}, nil
}

// Tries to claim a job from the runq. If none are available, return.
func (pd *Procd) getLambda() ([]byte, error) {
	err := pd.WaitForJob()
	if err != nil {
		return []byte{}, err
	}
	b, err := pd.getRun(proc.RUNQLC)
	if err != nil {
		return []byte{}, err
	}
	if len(b) != 0 {
		return b, nil
	}
	return pd.getRun(proc.RUNQ)
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
	pd.SignalNewJob()

	for !pd.readDone() {
		b, err := pd.getLambda()
		// If no job was on the runq, try again
		if err == nil && len(b) == 0 {
			continue
		}
		if err != nil && (err == io.EOF || strings.Contains(err.Error(), "unknown mount")) {
			continue
		}
		if err != nil {
			log.Fatalf("Procd GetLambda error %v, %v\n", err, b)
		}
		// XXX return err from spawn
		l, err := pd.spawn(b)
		if err != nil {
			log.Fatalf("Procd spawn error %v\n", err)
		}
		ls := []*Lambda{l}
		//		ls = append(ls, consumerLs...)
		pd.runAll(ls)
	}
	pd.SignalNewJob()
	pd.group.Done()
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

func unmarshalJob(b []byte) *proc.Proc {
	var p proc.Proc
	err := json.Unmarshal(b, &p)
	if err != nil {
		log.Fatalf("Procd couldn't unmarshal job: %v, %v", b, err)
		return nil
	}
	return &p
}
