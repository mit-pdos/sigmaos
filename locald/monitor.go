package locald

import (
	"encoding/json"
	"log"
	"path"
	"sync"
	"time"

	"ulambda/fslib"
	"ulambda/proc"
)

const (
	MONITOR_TIMER = 4
	CRASH_TIMEOUT = 1
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
	m.ProcCtl = proc.MakeProcCtl(fsl, m.pid)
	log.Printf("Monitor %v", m.pid)
	return m
}

// XXX Bring back continuations?
// Enqueue a new monitor to be run in MONITOR_TIMER seconds
func (m *Monitor) RestartSelf() {
	newPid := "monitor-" + fslib.GenPid()
	a := &proc.Proc{newPid, "bin/locald-monitor", "", []string{}, nil, nil, nil, MONITOR_TIMER, proc.T_LC, proc.C_DEF}
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

// Check if a job crashed. We know this is the case if it is fslib.CLAIMED, but
// the corresponding fslib.CLAIMED_EPH file is missing (locald crashed). Since, upon
// successful ClaimJob, there is a very short window during with the fslib.CLAIMED
// file exists but the fslib.CLAIMED_EPH file does not exist, wait a short amount of
// time (CRASH_TIMEOUT) before declaring a job as failed.
func (m *Monitor) JobCrashed(pid string) bool {
	_, err := m.Stat(path.Join(fslib.CLAIMED_EPH, pid))
	if err != nil {
		stat, err := m.Stat(path.Join(fslib.CLAIMED, pid))
		// If it has fully exited (both claimed & claimed_ephemeral are gone)
		if err != nil {
			return false
		}
		// If it is in the process of being claimed
		if int64(stat.Mtime+CRASH_TIMEOUT) < time.Now().Unix() {
			return true
		}
	}
	return false
}

// Move a job from fslib.CLAIMED to fslib.RUNQ
func (m *Monitor) RestartJob(pid string) error {
	// XXX read fslib.CLAIMED to find out if it is LC?
	b, _, err := m.GetFile(path.Join(fslib.CLAIMED, pid))
	if err != nil {
		return nil
	}
	var attr proc.Proc
	err = json.Unmarshal(b, &attr)
	if err != nil {
		log.Printf("Error unmarshalling in RestartJob: %v", err)
	}
	runq := fslib.RUNQ
	if attr.Type == proc.T_LC {
		runq = fslib.RUNQLC
	}
	m.Rename(path.Join(fslib.CLAIMED, pid), path.Join(runq, pid))
	// Notify localds that a job has become runnable
	m.SignalNewJob()
	return nil
}
