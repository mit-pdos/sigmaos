package locald

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
	"ulambda/stats"
)

const (
	NO_OP_LAMBDA = "no-op-lambda"
)

type LocalD struct {
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
	coresAvail fslib.Tcore
	srv        *npsrv.NpServer
	group      sync.WaitGroup
	perf       *perf.Perf
	st         *npo.SessionTable
	*fslib.FsLib
}

func MakeLocalD(bin string, pprofPath string, utilPath string) *LocalD {
	ld := &LocalD{}
	ld.cond = sync.NewCond(&ld.mu)
	ld.load = 0
	ld.nid = 0
	ld.bin = bin
	db.Name("locald")
	ld.root = ld.makeDir([]string{}, np.DMDIR, nil)
	ld.root.time = time.Now().Unix()
	ld.st = npo.MakeSessionTable()
	ld.ls = map[string]*Lambda{}
	ld.coreBitmap = make([]bool, linuxsched.NCores)
	ld.coresAvail = fslib.Tcore(linuxsched.NCores)
	ld.perf = perf.MakePerf()
	ip, err := fsclnt.LocalIP()
	ld.ip = ip
	if err != nil {
		log.Fatalf("LocalIP %v\n", err)
	}
	ld.srv = npsrv.MakeNpServer(ld, ld.ip+":0")
	fsl := fslib.MakeFsLib("locald")
	fsl.Mkdir(fslib.LOCALD_ROOT, 0777)
	ld.FsLib = fsl
	err = fsl.PostServiceUnion(ld.srv.MyAddr(), fslib.LOCALD_ROOT, ld.srv.MyAddr())
	if err != nil {
		log.Fatalf("locald PostServiceUnion failed %v %v\n", ld.srv.MyAddr(), err)
	}
	pprof := pprofPath != ""
	if pprof {
		ld.perf.SetupPprof(pprofPath)
	}
	// Must set core affinity before starting CPU Util measurements
	ld.setCoreAffinity()
	util := utilPath != ""
	if util {
		ld.perf.SetupCPUUtil(perf.CPU_UTIL_HZ, utilPath)
	}
	// Try to make scheduling directories if they don't already exist
	fsl.Mkdir(fslib.RUNQ, 0777)
	fsl.Mkdir(fslib.RUNQLC, 0777)
	fsl.Mkdir(fslib.WAITQ, 0777)
	fsl.Mkdir(fslib.CLAIMED, 0777)
	fsl.Mkdir(fslib.CLAIMED_EPH, 0777)
	fsl.Mkdir(fslib.SPAWNED, 0777)
	fsl.Mkdir(fslib.LOCKS, 0777)
	fsl.Mkdir(fslib.RET_STAT, 0777)
	fsl.Mkdir(fslib.TMP, 0777)
	return ld
}

func (ld *LocalD) spawn(a []byte) (*Lambda, error) {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	l := &Lambda{}
	l.ld = ld
	err := l.init(a)
	if err != nil {
		return nil, err
	}
	ld.ls[l.Pid] = l
	return l, nil
}

func (ld *LocalD) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(ld, conn)
}

func (ld *LocalD) Done() {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	ld.done = true
	ld.perf.Teardown()
	ld.SignalNewJob()
}

func (ld *LocalD) WatchTable() *npo.WatchTable {
	return nil
}

func (ld *LocalD) ConnTable() *npo.ConnTable {
	return nil
}

func (ld *LocalD) RegisterSession(sess np.Tsession) {
	ld.st.RegisterSession(sess)
}

func (ld *LocalD) SessionTable() *npo.SessionTable {
	return ld.st
}

func (ld *LocalD) Stats() *stats.Stats {
	return nil
}

func (ld *LocalD) readDone() bool {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	return ld.done
}

func (ld *LocalD) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return ld.root, nil
}

// XXX Statsd information?
// Check if this locald instance is able to satisfy a job's constraints.
// Trivially true when not benchmarking.
func (ld *LocalD) satisfiesConstraints(attr *fslib.Attr) bool {
	// Constraints are not checked when testing, as some tests require more cores
	// than we may have on our test machine.
	if !ld.perf.RunningBenchmark() {
		return true
	}
	// If we have enough cores to run this job...
	if ld.coresAvail >= attr.Ncore {
		return true
	}
	return false
}

// Update resource accounting information.
func (ld *LocalD) decrementResourcesL(attr *fslib.Attr) {
	ld.coresAvail -= attr.Ncore
}

// Update resource accounting information.
func (ld *LocalD) incrementResources(attr *fslib.Attr) {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	ld.incrementResourcesL(attr)
}
func (ld *LocalD) incrementResourcesL(attr *fslib.Attr) {
	ld.coresAvail += attr.Ncore
}

func (ld *LocalD) getRun(runq string) ([]byte, error) {
	jobs, err := ld.ReadRunQ(runq)
	if err != nil {
		return []byte{}, err
	}
	for _, j := range jobs {
		ld.mu.Lock()
		// Read a job
		b, err := ld.ReadJob(runq, j.Name)
		// If job has already been claimed, move on
		if err != nil {
			ld.mu.Unlock()
			continue
		}
		// Unmarshal it
		attr := unmarshalJob(b)
		// See if we can run it, and if so, try to claim it
		claimed := false
		if ld.satisfiesConstraints(attr) {
			b, claimed = ld.ClaimRunQJob(runq, j.Name)
		}
		if claimed {
			ld.decrementResourcesL(attr)
			ld.mu.Unlock()
			return b, nil
		}
		ld.mu.Unlock()
	}
	return []byte{}, nil
}

// Tries to claim a job from the runq. If none are available, return.
func (ld *LocalD) getLambda() ([]byte, error) {
	err := ld.WaitForJob()
	if err != nil {
		return []byte{}, err
	}
	b, err := ld.getRun(fslib.RUNQLC)
	if err != nil {
		return []byte{}, err
	}
	if len(b) != 0 {
		return b, nil
	}
	return ld.getRun(fslib.RUNQ)
}

// Scan through the waitq, and try to move jobs to the runq.
func (ld *LocalD) checkWaitingLambdas() {
	jobs, err := ld.ReadWaitQ()
	if err != nil {
		log.Fatalf("Error reading WaitQ: %v", err)
	}
	for _, j := range jobs {
		b, err := ld.ReadWaitQJob(j.Name)
		// Ignore errors: they may be frequent under high concurrency
		if err != nil || len(b) == 0 {
			continue
		}
		if ok, t := ld.jobIsRunnable(j, b); ok {
			// Ignore errors: they may be frequent under high concurrency
			ld.MarkJobRunnable(j.Name, t)
		}
	}
}

/*
 * 1. Timer-based lambdas are runnable after Mtime + attr.Timer > time.Now()
 * 2. ExitDep-based lambdas are runnable after all entries in the ExitDep map
 *    are true, whether that be because the dependencies explicitly exited or
 *    because they did not exist at spawn time (and were pruned).
 * 3. PairDep-based lambdas are runnable immediately if they are the producer,
 *    and after all producers have started running if they are the consumer. For
 *    now, we assume that the roles "Producer" and "Consumer" are mutually
 *    exclusive.
 *
 * *** For now, we assume the three "types" described above are mutually
 *    exclusive***
 */
func (ld *LocalD) jobIsRunnable(j *np.Stat, a []byte) (bool, fslib.Ttype) {
	var attr fslib.Attr
	err := json.Unmarshal(a, &attr)
	if err != nil {
		log.Printf("Couldn't unmarshal job to check if runnable %v: %v", a, err)
		return false, fslib.T_DEF
	}
	// If this is a timer-based lambda
	if attr.Timer != 0 {
		// If the timer has expired
		if uint32(time.Now().Unix()) > j.Mtime+attr.Timer {
			return true, attr.Type
		} else {
			// XXX Factor this out & do it in a monitor lambda
			// For now, just make sure *some* locald eventually wakes up to mark this
			// lambda as runnable. Otherwise, if there are only timer lambdas, localds
			// may never wake up to scan them.
			go func(timer uint32) {
				dur := time.Duration(uint64(timer) * 1000000000)
				time.Sleep(dur)
				ld.SignalNewJob()
			}(attr.Timer)
			return false, fslib.T_DEF
		}
	}

	// If this is a PairDep-based labmda
	if len(attr.PairDep) > 0 {
		// Update its pair deps
		// XXX CONTINUE HERE
		//		ld.updatePDeps(attr.Pid)
		for _, pair := range attr.PairDep {
			if attr.Pid == pair.Producer {
				return true, attr.Type
			} else if attr.Pid == pair.Consumer {
				// Someone will retry
				return false, fslib.T_DEF
			} else {
				log.Fatalf("Locald got PairDep-based lambda with lambda not in pair: %v, %v", attr.Pid, pair)
			}
		}
	}

	// If this is an ExitDep-based lambda
	for _, b := range attr.ExitDep {
		if !b {
			return false, fslib.T_DEF
		}
	}
	return true, attr.Type
}

func (ld *LocalD) spawnConsumers(bs [][]byte) []*Lambda {
	ls := []*Lambda{}
	for _, b := range bs {
		l, err := ld.spawn(b)
		if err != nil {
			log.Fatalf("Couldn't spawn consumer job: %v", string(b))
		}
		ls = append(ls, l)
	}
	return ls
}

func (ld *LocalD) allocCores(n fslib.Tcore) []uint {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	cores := []uint{}
	for i := 0; i < len(ld.coreBitmap); i++ {
		// If not running a benchmark or lambda asks for 0 cores, run on any core
		if !ld.perf.RunningBenchmark() || n == fslib.C_DEF {
			cores = append(cores, uint(i))
		} else {
			if !ld.coreBitmap[i] {
				ld.coreBitmap[i] = true
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

func (ld *LocalD) freeCores(cores []uint) {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	for _, i := range cores {
		ld.coreBitmap[i] = false
	}
}

func (ld *LocalD) runAll(ls []*Lambda) {
	var wg sync.WaitGroup
	for _, l := range ls {
		wg.Add(1)
		go func(l *Lambda) {
			defer wg.Done()

			// Allocate dedicated cores for this lambda to run on.
			cores := ld.allocCores(l.attr.Ncore)

			// Run the lambda.
			l.run(cores)

			// Free resources and dedicated cores.
			ld.incrementResources(l.attr)
			ld.freeCores(cores)

		}(l)
	}
	wg.Wait()
}

func (ld *LocalD) setCoreAffinity() {
	// XXX Currently, we just set the affinity for all available cores since Linux
	// seems to do a decent job of avoiding moving things around too much.
	m := &linuxsched.CPUMask{}
	for i := uint(0); i < linuxsched.NCores; i++ {
		m.Set(i)
	}
	// XXX For my current benchmarking setup, core 0 is reserved for ZK.
	if ld.perf.RunningBenchmark() {
		m.Clear(0)
		m.Clear(1)
	}
	linuxsched.SchedSetAffinityAllTasks(os.Getpid(), m)
}

// Worker runs one lambda at a time
func (ld *LocalD) Worker(workerId uint) {
	ld.SignalNewJob()

	for !ld.readDone() {
		b, err := ld.getLambda()
		// If no job was on the runq, try and move some from waitq -> runq
		if err == nil && len(b) == 0 {
			ld.checkWaitingLambdas()
			continue
		}
		if err != nil && (err == io.EOF || strings.Contains(err.Error(), "unknown mount")) {
			continue
		}
		if err != nil {
			log.Fatalf("Locald GetLambda error %v, %v\n", err, b)
		}
		// XXX return err from spawn
		l, err := ld.spawn(b)
		if err != nil {
			log.Fatalf("Locald spawn error %v\n", err)
		}
		//		// Try to claim, spawn, and run consumers if this lamba is a producer
		//		consumers := l.getConsumers()
		//		bs := ld.claimConsumers(consumers)
		//		consumerLs := ld.spawnConsumers(bs)
		ls := []*Lambda{l}
		//		ls = append(ls, consumerLs...)
		ld.runAll(ls)
	}
	ld.SignalNewJob()
	ld.group.Done()
}

func (ld *LocalD) Work() {
	var NWorkers uint
	// XXX May need a certain number of workers for tests, but need
	// NWorkers = NCores for benchmarks
	if !ld.perf.RunningBenchmark() && linuxsched.NCores < 20 {
		NWorkers = 20
	} else {
		NWorkers = linuxsched.NCores
		if ld.perf.RunningBenchmark() {
			NWorkers -= 1
		}
	}
	for i := uint(0); i < NWorkers; i++ {
		ld.group.Add(1)
		go ld.Worker(i)
	}
	ld.group.Wait()
}

func unmarshalJob(b []byte) *fslib.Attr {
	var attr fslib.Attr
	err := json.Unmarshal(b, &attr)
	if err != nil {
		log.Fatalf("Locald couldn't unmarshal job: %v, %v", b, err)
		return nil
	}
	return &attr
}
