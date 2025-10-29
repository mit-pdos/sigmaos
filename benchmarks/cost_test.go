package benchmarks_test

import (
	"fmt"
	"sync"
	"time"

	"sigmaos/proc"
	"sigmaos/util/perf"
)

const (
	MONITOR_COST_INTERVAL = 10 * time.Millisecond
)

type DeploymentCost struct {
	mu     sync.Mutex
	p      *perf.Perf
	ncores proc.Tmcpu
	done   bool
}

func NewDeploymentCost(p *perf.Perf) *DeploymentCost {
	return &DeploymentCost{
		p:      p,
		ncores: 0,
		done:   false,
	}
}

func (dc *DeploymentCost) String() string {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	return fmt.Sprintf("&{ ncores:%v done:%v }", dc.ncores, dc.done)
}

func (dc *DeploymentCost) AddNCore(n proc.Tmcpu) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.ncores += n
}

func (dc *DeploymentCost) RemoveNCore(n proc.Tmcpu) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.ncores -= n
}

func (dc *DeploymentCost) GetNCores() proc.Tmcpu {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	return dc.ncores
}

func (dc *DeploymentCost) isDone() bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	return dc.done
}

func (dc *DeploymentCost) monitorCost() {
	for !dc.isDone() {
		ncores := dc.GetNCores()
		dc.p.ValTick(float64(ncores))
		time.Sleep(MONITOR_COST_INTERVAL)
	}
}

func (dc *DeploymentCost) Run() {
	go dc.monitorCost()
}

func (dc *DeploymentCost) Stop() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.done = true
}
