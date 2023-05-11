package perf

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

//
// Perf output is controled by SIGMAPERF environment variable, which
// can be a list of labels (e.g., "PROCD_PPROF;NAMED_CPU;").
//

/*
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <stdlib.h>
*/
import "C"

func Hz() int {
	var chz C.long
	chz = C.sysconf(C._SC_CLK_TCK)
	h := int(chz)
	return h
}

const (
	OUTPUT_PATH            = "/tmp/sigmaos-perf"
	MUTEX_PROFILE_FRACTION = 1
	BLOCK_PROFILE_FRACTION = 5
)

type Tload [3]float64

func (t Tload) String() string {
	return fmt.Sprintf("[%.1f %.1f %.1f]", t[0], t[1], t[2])
}

var labels map[Tselector]bool

func init() {
	labelstr := proc.GetLabels(proc.SIGMAPERF)
	labels = make(map[Tselector]bool, len(labelstr))
	for k, v := range labelstr {
		labels[Tselector(k)] = v
	}
}

var loadfile *os.File

// XXX make into multiple structs
// Tracks performance statistics for any cores on which the current process is
// able to run.
type Perf struct {
	mu             sync.Mutex
	selector       Tselector
	done           uint32
	util           bool
	pprof          bool
	pprofMem       bool
	pprofMutex     bool
	pprofBlock     bool
	tpt            bool
	utilChan       chan bool
	utilFile       *os.File
	cpuCyclesBusy  []float64
	cpuCyclesTotal []float64
	cpuUtilPct     []float64
	cores          map[string]bool
	pprofFile      *os.File
	pprofMemFile   *os.File
	pprofMutexFile *os.File
	pprofBlockFile *os.File
	tpts           []float64
	times          []time.Time
	tptFile        *os.File
	sigc           chan os.Signal
}

func MakePerf(s Tselector) (*Perf, error) {
	return MakePerfMulti(s, "")
}

// A slight hack for benchmarks which wish to have 2 perf structures (one for
// each realm).
func MakePerfMulti(s Tselector, s2 string) (*Perf, error) {
	p := &Perf{}
	p.selector = s
	p.utilChan = make(chan bool, 1)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT, syscall.SIGABRT)
	go func() {
		<-sigc
		p.Done()
		os.Exit(143)
	}()
	// Make sure the PID is set (used to name the output files).
	if proc.GetPid().String() == "" {
		db.DFatalf("Must set PID before starting Perf")
	}
	// Make the output dir
	if err := os.MkdirAll(OUTPUT_PATH, 0777); err != nil {
		db.DPrintf(db.ALWAYS, "MakePerfMulti: MkdirAll %s err %v", OUTPUT_PATH, err)
		return nil, err
	}
	basePath := path.Join(OUTPUT_PATH, path.Base(proc.GetName()))
	if s2 != "" {
		basePath += "-" + s2
	}
	// Set up pprof caputre
	if ok := labels[s+PPROF]; ok {
		p.setupPprof(basePath + "-pprof.out")
	}
	// Set up pprof caputre
	if ok := labels[s+PPROF_MEM]; ok {
		p.setupPprofMem(basePath + "-pprof-mem.out")
	}
	// Set up pprof caputre
	if ok := labels[s+PPROF_MUTEX]; ok {
		p.setupPprofMutex(basePath + "-pprof-mutex.out")
	}
	// Set up pprof caputre
	if ok := labels[s+PPROF_BLOCK]; ok {
		p.setupPprofBlock(basePath + "-pprof-block.out")
	}
	// Set up cpu util capture
	if ok := labels[s+CPU]; ok {
		p.setupCPUUtil(sp.Conf.Perf.CPU_UTIL_SAMPLE_HZ, basePath+"-cpu.out")
	}
	// Set up throughput caputre
	if ok := labels[s+TPT]; ok {
		p.setupTpt(sp.Conf.Perf.CPU_UTIL_SAMPLE_HZ, basePath+"-tpt.out")
	}
	return p, nil
}

// Register that an event has happened with a given instantaneous throughput.
func (p *Perf) TptTick(tpt float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.tptTickL(tpt)
}

func (p *Perf) tptTickL(tpt float64) {
	// If we aren't recording throughput, return.
	if !p.tpt {
		return
	}

	// If it has been long enough since we started incrementing this slot, seal
	// it and move to the next slot. In this way, we always expect
	// len(p.times) == len(p.tpts) - 1
	if time.Since(p.times[len(p.times)-1]).Milliseconds() > int64(1000/sp.Conf.Perf.CPU_UTIL_SAMPLE_HZ) {
		p.tpts = append(p.tpts, 0.0)
		p.times = append(p.times, time.Now())
	}

	// Increment the current tpt slot.
	p.tpts[len(p.tpts)-1] += tpt
}

func (p *Perf) SumTicks() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.tpt {
		return 0.0
	}
	sum := float64(0)
	for _, tpt := range p.tpts {
		sum += tpt
	}
	return sum
}

func (p *Perf) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == 0 {
		atomic.StoreUint32(&p.done, 1)
		p.teardownPprof()
		p.teardownPprofMem()
		p.teardownPprofMutex()
		p.teardownPprofBlock()
		p.teardownUtil()
		p.teardownTpt()
	}
}

// Get the total cpu time usage for process with pid PID
func GetCPUTimePid(pid string) (utime, stime uint64, err error) {
	contents, err := ioutil.ReadFile(path.Join("/proc", pid, "stat"))
	if err != nil {
		db.DPrintf(db.ALWAYS, "Couldn't get CPU time: %v", err)
		return
	}
	fields := strings.Split(string(contents), " ")
	if len(fields) != 52 {
		db.DFatalf("Wrong num fields (%v): %v", len(fields), fields)
		return
	}
	// From: https://man7.org/linux/man-pages/man5/proc.5.html
	utime, err = strconv.ParseUint(fields[13], 10, 64)
	if err != nil {
		db.DFatalf("Error parse uint: %v", err)
		return
	}
	stime, err = strconv.ParseUint(fields[14], 10, 64)
	if err != nil {
		db.DFatalf("Error parse uint: %v", err)
		return
	}
	return
}

func GetCPUSample(cores map[string]bool) (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		db.DFatalf("Error read cpu util")
		return
	}
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if active, ok := cores[fields[0]]; ok && active {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					db.DPrintf(db.ALWAYS, "Error: %v %v %v", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle += val
				}
			}
		}
	}
	return
}

func GetLinuxLoad() Tload {
	// If load file isn't open, open it. Otherwise, seek to the beginning.
	if loadfile == nil {
		var err error
		loadfile, err = os.Open("/proc/loadavg")
		if err != nil {
			db.DFatalf("Couldn't open load file: %v", err)
		}
	} else {
		off, err := loadfile.Seek(0, 0)
		if err != nil || off != 0 {
			db.DFatalf("Error seeking in file: off %v err %v", off, err)
		}
	}
	b, err := ioutil.ReadAll(loadfile)
	if err != nil {
		db.DFatalf("Couldn't read load file: %v", err)
	}
	loadstr := strings.Split(string(b), " ")
	load := Tload{}
	load[0], err = strconv.ParseFloat(loadstr[0], 64)
	if err != nil {
		db.DFatalf("Couldn't parse float: %v", err)
	}
	load[1], err = strconv.ParseFloat(loadstr[1], 64)
	if err != nil {
		db.DFatalf("Couldn't parse float: %v", err)
	}
	load[2], err = strconv.ParseFloat(loadstr[2], 64)
	if err != nil {
		db.DFatalf("Couldn't parse float: %v", err)
	}
	return load
}

func UtilFromCPUTimeSample(utime0, stime0, utime1, stime1 uint64, secs float64) float64 {
	var total0 uint64
	var total1 uint64
	var delta float64
	var util float64
	var ticks float64

	total0 = utime0 + stime0
	total1 = utime1 + stime1
	delta = float64(total1 - total0)
	ticks = float64(Hz()) * secs
	util = 100.0 * delta / ticks
	return util
}

// Only count cycles on cores we can run on
func GetActiveCores() map[string]bool {
	// Get the cores we can run on
	m, err := linuxsched.SchedGetAffinity(os.Getpid())
	if err != nil {
		db.DFatalf("Error getting affinity mask: %v", err)
	}
	cores := map[string]bool{}
	for i := uint(0); i < linuxsched.NCores; i++ {
		if m.Test(i) {
			cores["cpu"+strconv.Itoa(int(i))] = true
		}
	}
	return cores
}

func (p *Perf) monitorCPUUtil(sampleHz int) {
	sleepMsecs := 1000 / sampleHz
	var idle0 uint64
	var total0 uint64
	var idle1 uint64
	var total1 uint64
	idx := 0
	idle0, total0 = GetCPUSample(p.cores)
	for atomic.LoadUint32(&p.done) != 1 {
		time.Sleep(time.Duration(sleepMsecs) * time.Millisecond)
		idle1, total1 = GetCPUSample(p.cores)
		idleDelta := float64(idle1 - idle0)
		totalDelta := float64(total1 - total0)
		util := 100.0 * (totalDelta - idleDelta) / totalDelta
		// Record number of cycles busy, utilized, and total
		p.cpuCyclesBusy = append(p.cpuCyclesBusy, totalDelta-idleDelta)
		p.cpuCyclesTotal = append(p.cpuCyclesTotal, totalDelta)
		p.cpuUtilPct = append(p.cpuUtilPct, util)
		idx += 1
		idle0 = idle1
		total0 = total1
	}
	// Trim preallocated
	p.cpuCyclesBusy = p.cpuCyclesBusy[:idx]
	p.cpuCyclesTotal = p.cpuCyclesTotal[:idx]
	p.cpuUtilPct = p.cpuUtilPct[:idx]
	p.utilChan <- true
}

func (p *Perf) setupCPUUtil(sampleHz int, fpath string) {
	p.mu.Lock()

	p.util = true
	f, err := os.Create(fpath)
	if err != nil {
		db.DFatalf("Create util file %v failed %v", fpath, err)
	}
	p.utilFile = f
	// TODO: pre-allocate a large number of entries
	p.cpuCyclesBusy = make([]float64, 0, 40*sampleHz)
	p.cpuCyclesTotal = make([]float64, 0, 40*sampleHz)
	p.cpuUtilPct = make([]float64, 0, 40*sampleHz)
	p.cores = GetActiveCores()

	p.mu.Unlock()

	go p.monitorCPUUtil(sampleHz)
}

func (p *Perf) setupTpt(sampleHz int, fpath string) {
	p.mu.Lock()

	p.tpt = true
	f, err := os.Create(fpath)
	if err != nil {
		db.DFatalf("Create tpt file %v failed %v", fpath, err)
	}
	p.tptFile = f
	// Pre-allocate a large number of entries (40 secs worth)
	p.times = make([]time.Time, 0, 40*sampleHz)
	p.tpts = make([]float64, 0, 40*sampleHz)

	p.times = append(p.times, time.Now())
	p.tpts = append(p.tpts, 0.0)

	p.mu.Unlock()
}

func (p *Perf) setupPprof(fpath string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	f, err := os.Create(fpath)
	if err != nil {
		db.DFatalf("Couldn't create pprof profile file: %v, %v", fpath, err)
	}
	p.pprof = true
	p.pprofFile = f
	if err := pprof.StartCPUProfile(f); err != nil {
		debug.PrintStack()
		db.DFatalf("Couldn't start CPU profile: %v", err)
	}
}

func (p *Perf) setupPprofMem(fpath string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	f, err := os.Create(fpath)
	if err != nil {
		db.DFatalf("Couldn't create pprofMem profile file: %v, %v", fpath, err)
	}
	p.pprofMem = true
	p.pprofMemFile = f
}

func (p *Perf) setupPprofMutex(fpath string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	runtime.SetMutexProfileFraction(MUTEX_PROFILE_FRACTION)

	f, err := os.Create(fpath)
	if err != nil {
		db.DFatalf("Couldn't create pprofMutex profile file: %v, %v", fpath, err)
	}
	p.pprofMutex = true
	p.pprofMutexFile = f
}

func (p *Perf) setupPprofBlock(fpath string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	runtime.SetBlockProfileRate(BLOCK_PROFILE_FRACTION)

	f, err := os.Create(fpath)
	if err != nil {
		db.DFatalf("Couldn't create pprofBlock profile file: %v, %v", fpath, err)
	}
	p.pprofBlock = true
	p.pprofBlockFile = f
}

// Caller holds lock.
func (p *Perf) teardownPprof() {
	if p.pprof {
		// Avoid double-closing
		p.pprof = false
		pprof.StopCPUProfile()
		if err := p.pprofFile.Sync(); err != nil {
			db.DFatalf("Error sync pprof file: %v", err)
		}
		if err := p.pprofFile.Close(); err != nil {
			db.DFatalf("Error close pprof file: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Done flushing and closing pprof file")
	}
}

// Caller holds lock.
func (p *Perf) teardownPprofMem() {
	if p.pprofMem {
		// Avoid double-closing
		p.pprofMem = false
		// Don't do GC before collecting the heap profile.
		// runtime.GC() // get up-to-date statistics
		// Write a heap profile
		if err := pprof.WriteHeapProfile(p.pprofMemFile); err != nil {
			db.DFatalf("could not write memory profile: %v", err)
		}
		p.pprofMemFile.Close()
	}
}

func (p *Perf) teardownPprofMutex() {
	if p.pprofMutex {
		// Avoid double-closing
		p.pprofMutex = false
		// Don't do GC before collecting the heap profile.
		// runtime.GC() // get up-to-date statistics
		// Write a heap profile
		if err := pprof.Lookup("mutex").WriteTo(p.pprofMutexFile, 0); err != nil {
			db.DFatalf("could not write mutex profile: %v", err)
		}
		p.pprofMutexFile.Close()
	}
}

func (p *Perf) teardownPprofBlock() {
	if p.pprofBlock {
		// Avoid double-closing
		p.pprofBlock = false
		// Don't do GC before collecting the heap profile.
		// runtime.GC() // get up-to-date statistics
		// Write a heap profile
		if err := pprof.Lookup("mutex").WriteTo(p.pprofBlockFile, 0); err != nil {
			db.DFatalf("could not write mutex profile: %v", err)
		}
		p.pprofBlockFile.Close()
	}
}

// Caller holds lock.
func (p *Perf) teardownUtil() {
	if p.util {
		<-p.utilChan
		// Avoid double-closing
		p.util = false
		for i := 0; i < len(p.cpuCyclesBusy); i++ {
			if _, err := p.utilFile.WriteString(fmt.Sprintf("%f,%f,%f\n", p.cpuUtilPct[i], p.cpuCyclesBusy[i], p.cpuCyclesTotal[i])); err != nil {
				db.DFatalf("Error writing to util file: %v", err)
			}
		}
	}
}

// Caller holds lock.
func (p *Perf) teardownTpt() {
	if p.tpt {
		p.tpt = false
		// Ignore first entry.
		for i := 0; i < len(p.times); i++ {
			if _, err := p.tptFile.WriteString(fmt.Sprintf("%vus,%f\n", p.times[i].UnixMicro(), p.tpts[i])); err != nil {
				db.DFatalf("Error writing to tpt file: %v", err)
			}
		}
	}
}
