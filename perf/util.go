package perf

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"ulambda/linuxsched"
)

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

// XXX delete? use Hz()
const (
	CPU_UTIL_HZ = 10
)

// Tracks performance statistics for any cores on which the current process is
// able to run.
type Perf struct {
	mu             sync.Mutex
	done           uint32
	util           bool
	utilChan       chan bool
	utilPath       string
	utilFile       *os.File
	cpuCyclesBusy  []float64
	cpuCyclesTotal []float64
	cpuUtilPct     []float64
	pprof          bool
	pprofPath      string
	pprofFile      *os.File
	cores          map[string]bool
	sigc           chan os.Signal
}

func MakePerf() *Perf {
	p := &Perf{}
	p.cores = map[string]bool{}
	p.utilChan = make(chan bool, 1)
	sigc := make(chan os.Signal, 1)
	log.Printf("Notify")
	signal.Notify(sigc, os.Interrupt, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT, syscall.SIGABRT)
	go func() {
		<-sigc
		p.Teardown()
		os.Exit(0)
	}()
	return p
}

// Read utime+stime from /proc/<pid>/stat
func GetPIDSample(pid int) uint64 {
	contents, err := ioutil.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		log.Printf("Error reading /proc/<pid>/stat %v", err)
	}
	line := strings.Split(strings.TrimSpace(string(contents)), " ")
	utime, err := strconv.ParseUint(line[13], 10, 32)
	if err != nil {
		log.Printf("Error: utime %v", err)
	}
	stime, err := strconv.ParseUint(line[14], 10, 32)
	if err != nil {
		log.Printf("Error: stime %v", err)
	}
	return utime + stime
}

func GetCPUSample(cores map[string]bool) (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
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
					log.Printf("Error: %v %v %v", i, fields[i], err)
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

func (p *Perf) RunningBenchmark() bool {
	return p.util || p.pprof
}

func (p *Perf) monitorCPUUtil(hz int) {
	sleepMsecs := 1000 / hz
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
		// log.Printf("CPU util: %f [busy: %f, total: %f]\n", util, totalDelta-idleDelta, totalDelta)
		// Record number of cycles busy, utilized, and total
		if idx < 40*CPU_UTIL_HZ {
			p.cpuCyclesBusy[idx] = totalDelta - idleDelta
			p.cpuCyclesTotal[idx] = totalDelta
			p.cpuUtilPct[idx] = util
		} else {
			p.cpuCyclesBusy = append(p.cpuCyclesBusy, totalDelta-idleDelta)
			p.cpuCyclesTotal = append(p.cpuCyclesTotal, totalDelta)
			p.cpuUtilPct = append(p.cpuUtilPct, util)
		}
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

// Only count cycles on cores we can run on
func (p *Perf) getActiveCores() {
	linuxsched.ScanTopology()
	// Get the cores we can run on
	m, err := linuxsched.SchedGetAffinity(os.Getpid())
	if err != nil {
		log.Fatalf("Error getting affinity mask: %v", err)
	}
	for i := uint(0); i < linuxsched.NCores; i++ {
		if m.Test(i) {
			p.cores["cpu"+strconv.Itoa(int(i))] = true
		}
	}
}

func (p *Perf) SetupCPUUtil(hz int, fpath string) {
	p.mu.Lock()

	p.util = true
	p.utilPath = fpath
	f, err := os.Create(p.utilPath)
	if err != nil {
		log.Fatalf("Create util file %v failed %v", p.utilPath, err)
	}
	p.utilFile = f
	// TODO: pre-allocate a large number of entries
	p.cpuCyclesBusy = make([]float64, 40*CPU_UTIL_HZ)
	p.cpuCyclesTotal = make([]float64, 40*CPU_UTIL_HZ)
	p.cpuUtilPct = make([]float64, 40*CPU_UTIL_HZ)
	p.getActiveCores()

	p.mu.Unlock()

	go p.monitorCPUUtil(hz)
}

func (p *Perf) SetupPprof(fpath string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	f, err := os.Create(fpath)
	if err != nil {
		log.Fatalf("Couldn't create pprof profile file: %v, %v", fpath, err)
	}
	p.pprof = true
	p.pprofPath = fpath
	p.pprofFile = f
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatalf("Couldn't start CPU profile: %v", err)
	}
}

func (p *Perf) Teardown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == 0 {
		atomic.StoreUint32(&p.done, 1)
		p.teardownPprof()
		p.teardownUtil()
	}
}

func (p *Perf) teardownPprof() {
	if p.pprof {
		// Avoid double-closing
		p.pprof = false
		pprof.StopCPUProfile()
		p.pprofFile.Close()
	}
}

func (p *Perf) teardownUtil() {
	if p.util {
		<-p.utilChan
		// Avoid double-closing
		p.util = false
		for i := 0; i < len(p.cpuCyclesBusy); i++ {
			if _, err := p.utilFile.WriteString(fmt.Sprintf("%f,%f,%f\n", p.cpuUtilPct[i], p.cpuCyclesBusy[i], p.cpuCyclesTotal[i])); err != nil {
				log.Fatalf("Error writing to util file: %v", err)
			}
		}
	}
}
