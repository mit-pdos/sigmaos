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

	dc.p.ValTick(float64(dc.ncores))
	dc.ncores += n
	dc.p.ValTick(float64(dc.ncores))
}

func (dc *DeploymentCost) RemoveNCore(n proc.Tmcpu) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.p.ValTick(float64(dc.ncores))
	dc.ncores -= n
	dc.p.ValTick(float64(dc.ncores))
}

func (dc *DeploymentCost) GetNCores() proc.Tmcpu {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	return dc.ncores
}
