package idemproc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
	"ulambda/sync"
)

const (
	IDEMPROC_LOCK = "idemproc-lock"
)

type Monitor struct {
	pid string
	l   *sync.Lock
	*fslib.FsLib
	*IdemProcCtl
}

func MakeMonitor(args []string) *Monitor {
	m := &Monitor{}
	m.pid = args[0]
	m.FsLib = fslib.MakeFsLib(m.pid)
	m.l = sync.MakeLock(m.FsLib, fslib.LOCKS, IDEMPROC_LOCK, true)
	m.IdemProcCtl = MakeIdemProcCtl(m.FsLib)
	db.Name(m.pid)

	log.Printf("Monitor: %v", m)

	m.Started(m.pid)
	return m
}

func (m *Monitor) waitEvict() {
	err := m.WaitEvict(m.pid)
	if err != nil {
		log.Fatalf("Error WaitEvict: %v", err)
	}
	m.Exit()
	os.Exit(0)
}

func (m *Monitor) watchProcds() {
	log.Printf("Monitor %v set watch", m)
	done := make(chan bool)
	err := m.SetDirWatch(kernel.PROCD, func(p string, err error) {
		if err != nil && err.Error() == "EOF" {
			return
		} else if err != nil {
			log.Printf("Error SetDirWatch in idemproc.Monitor.watchProcds: %v", err)
			db.DLPrintf("MONITOR", "Error DirWatch in idemproc.Monitor.watchProcds: %v", err)
		}
		done <- true
	})

	// If error, don't wait.
	if err == nil {
		<-done
	} else {
		log.Printf("Error SetDirWatch in idemproc.Monitor.watchProcds: %v", err)
		db.DLPrintf("MONITOR", "Error SetDirWatch in idemproc.Monitor.watchProcds: %v", err)
	}
}

func (m *Monitor) getFailedProcds() []string {
	remaining, err := m.ReadDir(kernel.PROCD)
	if err != nil {
		log.Fatalf("Error ReadDir 1 in Monitor.getFailedProcs: %v", err)
	}

	procdIPs := map[string]bool{}
	for _, r := range remaining {
		procdIPs[r.Name] = true
	}

	oldProcds, err := m.ReadDir(IDEM_PROCS)
	if err != nil {
		log.Fatalf("Error ReadDir 2 in Monitor.getFailedProcs: %v", err)
	}

	failedProcds := []string{}
	for _, o := range oldProcds {
		if _, ok := procdIPs[o.Name]; !ok {
			failedProcds = append(failedProcds, o.Name)
		}
	}
	return failedProcds
}

func (m *Monitor) getFailedProcs() []*IdemProc {
	failedProcds := m.getFailedProcds()
	failedProcs := []*IdemProc{}
	for _, procdIP := range failedProcds {
		procs, err := m.ReadDir(path.Join(IDEM_PROCS, procdIP))
		if err != nil {
			log.Fatalf("Error ReadDir 3 in Monitor.getFailedProcs: %v", err)
		}
		for _, p := range procs {
			failedProcs = append(failedProcs, m.getProc(procdIP, p.Name))
		}
	}
	return failedProcs
}

func (m *Monitor) getProc(procdIP string, pid string) *IdemProc {
	b, err := m.ReadFile(idemProcFilePath(procdIP, pid))
	if err != nil {
		log.Fatalf("Error ReadFile in Monitor.getProc: %v", err)
	}

	p := &proc.Proc{}
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Error Unmarshal in Monitor.getProc: %v", err)
	}
	return &IdemProc{p}
}

func (m *Monitor) respawnFailedProcs(ps []*IdemProc) {
	for _, p := range ps {
		m.Spawn(p)
	}
}

func (m *Monitor) Work() {
	go m.waitEvict()
	for {
		m.watchProcds()
		if ok := m.l.TryLock(); !ok {
			continue
		}
		ps := m.getFailedProcs()
		m.respawnFailedProcs(ps)
		// XXX clean up old files & dirs
	}
}

func (m *Monitor) Exit() {
	m.Exited(m.pid)
}

func (m *Monitor) String() string {
	return fmt.Sprintf("&{ pid:%v }", m.pid)
}
