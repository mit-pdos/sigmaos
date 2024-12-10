package cpumon

import (
	"sync"
	"sync/atomic"
	"time"

	"sigmaos/util/perf"
	"sigmaos/sigmasrv/stats"
)

type UtilFn func() float64

type CpuMon struct {
	mu            sync.Mutex
	monitoringCPU bool
	done          uint32
	cores         map[string]bool
	hz            int
	st            *stats.StatInode

	util float64

	load       perf.Tload
	customLoad perf.Tload
}

func NewCpuMon(st *stats.StatInode, ufn UtilFn) *CpuMon {
	cm := &CpuMon{}
	cm.st = st
	cm.hz = perf.Hz()
	// Don't duplicate work
	if !cm.monitoringCPU {
		cm.monitoringCPU = true
		go cm.monitorCPUUtil(ufn)
	}
	return cm
}

const (
	EXP_0 = 0.9048 // 1/exp(100ms/1000ms)
	EXP_1 = 0.9512 // 1/exp(100ms/2000ms)
	EXP_2 = 0.9801 // 1/exp(100ms/5000ms)
	MS    = 100    // 100 ms
	SEC   = 1000   // 1s
)

func setLoad(load *perf.Tload, x float64) {
	load[0] *= EXP_0
	load[0] += (1 - EXP_0) * x
	load[1] *= EXP_1
	load[1] += (1 - EXP_1) * x
	load[2] *= EXP_2
	load[2] += (1 - EXP_2) * x
}

// Caller holds lock
func (cm *CpuMon) loadCPUUtilL(idle, total uint64, customUtil float64) {
	util := 100.0 * (1.0 - float64(idle)/float64(total))

	setLoad(&cm.load, util)
	setLoad(&cm.customLoad, customUtil)
	cm.util = util
	cm.st.SetLoad(cm.load, cm.customLoad, util, customUtil)
}

// Update the set of cores we scan to determine CPU utilization.
func (cm *CpuMon) UpdateCores() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.cores = perf.GetActiveCores()
}

func (cm *CpuMon) monitorCPUUtil(ufn UtilFn) {
	total0 := uint64(0)
	total1 := uint64(0)
	idle0 := uint64(0)
	idle1 := uint64(0)

	cm.UpdateCores()

	for atomic.LoadUint32(&cm.done) != 1 {
		time.Sleep(time.Duration(MS) * time.Millisecond)

		// Can't call into ufn while the st lock is held, in order to ensure lock
		// ordering and avoid deadlock.
		var customUtil float64
		if ufn != nil {
			customUtil = ufn()
		}
		// Lock in case the set of cores we're monitoring changes.
		cm.mu.Lock()
		idle1, total1 = perf.GetCPUSample(cm.cores)
		cm.loadCPUUtilL(idle1-idle0, total1-total0, customUtil)
		cm.mu.Unlock()

		total0 = total1
		idle0 = idle1
	}
}

func (cm *CpuMon) Done() {
	atomic.StoreUint32(&cm.done, 1)
}

func (cm *CpuMon) GetUtil() (float64, float64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.util, 0.0
}

func (cm *CpuMon) GetLoad() perf.Tload {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	load := perf.Tload{}
	load[0] = cm.load[0]
	load[1] = cm.load[1]
	load[2] = cm.load[2]
	return load
}
