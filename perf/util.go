package perf

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	db "ulambda/debug"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
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
	OUTPUT_PATH = np.UXROOT + "perf-output/"
	PPROF       = "_PPROF"
	CPU         = "_CPU"
	TPT         = "_TPT"
)

var labels map[string]bool

func init() {
	labels = proc.GetLabels("SIGMAPERF")
}

// XXX make into multiple structs
// Tracks performance statistics for any cores on which the current process is
// able to run.
type Perf struct {
	mu             sync.Mutex
	name           string
	done           uint32
	util           bool
	pprof          bool
	tpt            bool
	utilChan       chan bool
	utilFile       *os.File
	cpuCyclesBusy  []float64
	cpuCyclesTotal []float64
	cpuUtilPct     []float64
	cores          map[string]bool
	pprofFile      *os.File
	tpts           []float64
	times          []time.Time
	tptFile        *os.File
	sigc           chan os.Signal
}

func MakePerf(name string) *Perf {
	p := &Perf{}
	p.name = name
	p.utilChan = make(chan bool, 1)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT, syscall.SIGABRT)
	go func() {
		<-sigc
		p.Done()
		os.Exit(0)
	}()
	// Make sure the PID is set (used to name the output files).
	if proc.GetPid().String() == "" {
		db.DFatalf("Must set PID before starting Perf")
	}
	// Make the output dir
	if err := os.MkdirAll(OUTPUT_PATH, 0777); err != nil {
		db.DFatalf("Error Mkdir: %v", err)
	}
	// Set up pprof caputre
	if ok := labels[name+PPROF]; ok {
		p.setupPprof(path.Join(OUTPUT_PATH, proc.GetName()+"-"+proc.GetPid().String()+"-pprof.out"))
	}
	// Set up cpu util capture
	if ok := labels[name+CPU]; ok {
		p.setupCPUUtil(np.Conf.Perf.CPU_UTIL_SAMPLE_HZ, path.Join(OUTPUT_PATH, proc.GetName()+"-"+proc.GetPid().String()+"-cpu.out"))
	}
	// Set up throughput caputre
	if ok := labels[name+TPT]; ok {
		p.setupTpt(np.Conf.Perf.CPU_UTIL_SAMPLE_HZ, path.Join(OUTPUT_PATH, proc.GetName()+"-"+proc.GetPid().String()+"-tpt.out"))
	}
	return p
}

// Register that an event has happened with a given instantaneous throughput.
func (p *Perf) TptTick(tpt float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If we aren't recording throughput, return.
	if !p.tpt {
		return
	}

	// Increment the current tpt slot.
	p.tpts[len(p.tpts)-1] += tpt

	// If it has been long enough since we started incrementing this slot, seal
	// it and move on. In this way, we always expect
	// len(p.times) == len(p.tpts) - 1
	if time.Since(p.times[len(p.times)-1]).Milliseconds() > int64(1000/np.Conf.Perf.CPU_UTIL_SAMPLE_HZ) {
		p.tpts = append(p.tpts, 0.0)
		p.times = append(p.times, time.Now())
	}
}

func (p *Perf) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == 0 {
		atomic.StoreUint32(&p.done, 1)
		p.teardownPprof()
		p.teardownUtil()
		p.teardownTpt()
	}
}

// Get the total cpu time usage for process with pid PID
func GetCPUTimePid(pid string) (utime, stime uint64) {
	contents, err := ioutil.ReadFile(path.Join("/proc", pid, "stat"))
	if err != nil {
		return
	}
	fields := strings.Split(string(contents), " ")
	if len(fields) != 52 {
		db.DFatalf("Wrong num fields (%v): %v", len(fields), fields)
	}
	// From: https://man7.org/linux/man-pages/man5/proc.5.html
	utime, err = strconv.ParseUint(fields[13], 10, 64)
	if err != nil {
		db.DFatalf("Error parse uint: %v", err)
	}
	stime, err = strconv.ParseUint(fields[14], 10, 64)
	if err != nil {
		db.DFatalf("Error parse uint: %v", err)
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

// XXX Performance impact?
func (p *Perf) setupTpt(sampleHz int, fpath string) {
	p.mu.Lock()

	p.tpt = true
	f, err := os.Create(fpath)
	if err != nil {
		db.DFatalf("Create tpt file %v failed %v", fpath, err)
	}
	p.tptFile = f
	// TODO: pre-allocate a large number of entries
	p.times = make([]time.Time, 1, 40*sampleHz)
	p.tpts = make([]float64, 2, 40*sampleHz)

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
		db.DFatalf("Couldn't start CPU profile: %v", err)
	}
}

// Caller holds lock.
func (p *Perf) teardownPprof() {
	if p.pprof {
		// Avoid double-closing
		p.pprof = false
		pprof.StopCPUProfile()
		p.pprofFile.Close()
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
		for i := 1; i < len(p.times); i++ {
			if _, err := p.utilFile.WriteString(fmt.Sprintf("%vus,%f\n", p.times[i].UnixMicro(), p.tpts[i])); err != nil {
				db.DFatalf("Error writing to util file: %v", err)
			}
		}
	}
}
