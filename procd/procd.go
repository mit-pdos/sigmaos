package procd

import (
	"io"
	"log"
	"net"
	"os"
	"path"
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
	cond       *sync.Cond
	jobLock    *usync.Lock
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
	fsl.Mkdir(PROCD_ROOT, 0777)
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
	// Set up the job lock
	pd.jobLock = usync.MakeLock(fsl, fslib.LOCKS, proc.JOB_SIGNAL, false)
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

func (pd *Procd) WaitForJob() {
	// Wait for something runnable
	pd.jobLock.Lock()
}

// Claim a job by moving it from the runq to the claimed dir
func (pd *Procd) ClaimRunQJob(queuePath string, pid string) bool {
	// Write the file to reset its mtime (to avoid racing with Monitor). Ignore
	// errors in the event we lose the race.
	pd.WriteFile(path.Join(queuePath, pid), []byte{})
	err := pd.Rename(path.Join(queuePath, pid), path.Join(proc.CLAIMED, pid))
	if err != nil {
		return false
	}
	// Create an ephemeral file to mark that procd hasn't crashed
	err = pd.MakeFile(path.Join(proc.CLAIMED_EPH, pid), 0777|np.DMTMP, np.OWRITE, []byte{})
	if err != nil {
		log.Printf("Error making ephemeral claimed job file: %v", err)
	}
	_, _, err = pd.GetFile(path.Join(proc.CLAIMED, pid))
	if err != nil {
		log.Printf("Error reading claimed job: %v", err)
		return false
	}
	// We shouldn't hold the "new job" lock while running a lambda/doing work
	pd.SignalNewJob()
	return true
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

func (pd *Procd) getRun(runq string) (*proc.Proc, error) {
	jobs, err := pd.ReadDir(runq)
	if err != nil {
		return nil, err
	}
	for _, j := range jobs {
		pd.mu.Lock()
		// Read a job
		p, err := pd.GetProcFile(runq, j.Name)
		// If job has already been claimed, move on
		if err != nil {
			pd.mu.Unlock()
			continue
		}
		// See if we can run it, and if so, try to claim it
		if pd.satisfiesConstraints(p) {
			if ok := pd.ClaimRunQJob(runq, j.Name); ok {
				pd.decrementResourcesL(p)
				pd.mu.Unlock()
				return p, nil
			}
		}
		pd.mu.Unlock()
	}
	return nil, nil
}

// Tries to claim a job from the runq. If none are available, return.
func (pd *Procd) getProc() (*proc.Proc, error) {
	pd.WaitForJob()
	//	if err != nil {
	//		return nil, err
	//	}
	p, err := pd.getRun(proc.RUNQLC)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
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
		p, err := pd.getProc()
		// If no job was on the runq, try again
		if err == nil && p == nil {
			continue
		}
		if err != nil && (err == io.EOF || strings.Contains(err.Error(), "unknown mount")) {
			continue
		}
		if err != nil {
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
