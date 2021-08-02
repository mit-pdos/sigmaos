package locald

import (
	"log"
	"sync"

	"ulambda/fslib"
	"ulambda/proc"
)

const (
	MONITOR_TIMER = 4
)

// Monitors for crashed localds/lambdas
type Monitor struct {
	mu  sync.Mutex
	pid string
	*fslib.FsLib
	*proc.ProcCtl
}

func MakeMonitor(pid string) *Monitor {
	m := &Monitor{}
	m.pid = pid
	fsl := fslib.MakeFsLib("monitor")
	m.FsLib = fsl
	m.ProcCtl = proc.MakeProcCtl(fsl)
	log.Printf("Monitor %v", m.pid)
	return m
}

// XXX Bring back continuations?
// Enqueue a new monitor to be run in MONITOR_TIMER seconds
func (m *Monitor) RestartSelf() {
	newPid := "monitor-" + fslib.GenPid()
	a := &fslib.Attr{newPid, "bin/locald-monitor", "", []string{}, nil, nil, nil, MONITOR_TIMER, fslib.T_LC, fslib.C_DEF}
	err := m.Spawn(a)
	if err != nil {
		log.Printf("Error spawning monitor: %v", err)
	}
}

func (m *Monitor) Work() {
	jobs, err := m.ReadClaimed()
	if err != nil {
		log.Printf("Error in monitor's ReadClaimed: %v", err)
	}
	for _, j := range jobs {
		if m.JobCrashed(j.Name) {
			m.RestartJob(j.Name)
		}
	}
	m.RestartSelf()
}
