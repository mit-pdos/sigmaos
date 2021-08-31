package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/idemproc"
	"ulambda/kernel"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/sync"
)

const (
	IDEMPROC_LOCK = "idemproc-lock"
)

type ProcdMonitor struct {
	pid string
	l   *sync.Lock
	*fslib.FsLib
	proc.ProcCtl
}

func MakeProcdMonitor(args []string) *ProcdMonitor {
	m := &ProcdMonitor{}
	m.pid = args[0]
	m.FsLib = fslib.MakeFsLib(m.pid)
	m.l = sync.MakeLock(m.FsLib, named.LOCKS, IDEMPROC_LOCK, true)
	m.ProcCtl = procinit.MakeProcCtl(m.FsLib, procinit.GetProcLayersMap())
	db.Name(m.pid)

	log.Printf("ProcdMonitor: %v", m)

	m.Started(m.pid)
	return m
}

func (m *ProcdMonitor) waitEvict() {
	err := m.WaitEvict(m.pid)
	if err != nil {
		log.Fatalf("Error WaitEvict: %v", err)
	}
	m.Exit()
	os.Exit(0)
}

func (m *ProcdMonitor) watchProcds() {
	log.Printf("ProcdMonitor %v set watch", m)
	done := make(chan bool)
	err := m.SetDirWatch(kernel.PROCD, func(p string, err error) {
		if err != nil && err.Error() == "EOF" {
			return
		} else if err != nil {
			log.Printf("Error SetDirWatch in idemproc.ProcdMonitor.watchProcds: %v", err)
			db.DLPrintf("MONITOR", "Error DirWatch in idemproc.ProcdMonitor.watchProcds: %v", err)
		}
		done <- true
	})

	// If error, don't wait.
	if err == nil {
		<-done
	} else {
		log.Printf("Error SetDirWatch in idemproc.ProcdMonitor.watchProcds: %v", err)
		db.DLPrintf("MONITOR", "Error SetDirWatch in idemproc.ProcdMonitor.watchProcds: %v", err)
	}
}

// Read & unmarshal a proc.
func (m *ProcdMonitor) getProc(procdIP string, pid string) *idemproc.IdemProc {
	b, err := m.ReadFile(idemproc.IdemProcFilePath(procdIP, pid))
	if err != nil {
		log.Fatalf("Error ReadFile in ProcdMonitor.getProc: %v", err)
	}

	p := &proc.Proc{}
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Error Unmarshal in ProcdMonitor.getProc: %v", err)
	}
	return &idemproc.IdemProc{p}
}

// Get a list of the failed procds.
func (m *ProcdMonitor) getFailedProcds() []string {
	remaining, err := m.ReadDir(kernel.PROCD)
	if err != nil {
		log.Fatalf("Error ReadDir 1 in ProcdMonitor.getFailedProcds: %v", err)
	}

	procdIPs := map[string]bool{}
	for _, r := range remaining {
		procdIPs[r.Name] = true
	}

	oldProcds, err := m.ReadDir(idemproc.IDEM_PROCS)
	if err != nil {
		log.Fatalf("Error ReadDir 2 in ProcdMonitor.getFailedProcds: %v", err)
	}

	failedProcds := []string{}
	for _, o := range oldProcds {
		if _, ok := procdIPs[o.Name]; !ok && o.Name != idemproc.NEED_RESTART {
			failedProcds = append(failedProcds, o.Name)
		}
	}
	return failedProcds
}

// Moves procs from failed procd directory to idemproc.NEED_RESTART directory.
func (m *ProcdMonitor) markProcsNeedRestart() {
	failedProcds := m.getFailedProcds()
	for _, procdIP := range failedProcds {
		procs, err := m.ReadDir(path.Join(idemproc.IDEM_PROCS, procdIP))
		if err != nil {
			log.Fatalf("Error ReadDir in ProcdMonitor.markProcsNeedRestart: %v", err)
		}
		for _, p := range procs {
			old := idemproc.IdemProcFilePath(procdIP, p.Name)
			new := idemproc.IdemProcFilePath(idemproc.NEED_RESTART, p.Name)
			err := m.Rename(old, new)
			if err != nil {
				log.Fatalf("Error rename in ProcdMonitor.markProcsNeedRestart: %v", err)
			}
		}
	}
}

// Retrieves procs from idemproc.NEED_RESTART directory.
func (m *ProcdMonitor) getProcsNeedRestart() []*idemproc.IdemProc {
	needRestart := []*idemproc.IdemProc{}
	procs, err := m.ReadDir(path.Join(idemproc.IDEM_PROCS, idemproc.NEED_RESTART))
	if err != nil {
		log.Fatalf("Error ReadDir in ProcdMonitor.getProcsNeedRestart: %v", err)
	}
	for _, p := range procs {
		needRestart = append(needRestart, m.getProc(idemproc.NEED_RESTART, p.Name))
	}
	return needRestart
}

// Respawn procs which may need a restart.
func (m *ProcdMonitor) respawnProcs(ps []*idemproc.IdemProc) {
	for _, p := range ps {
		err := m.Spawn(p)
		if err != nil {
			log.Fatalf("Error Spawn in ProcdMonitor.respawnFailedProcs: %v", err)
		}
		err = m.Remove(idemproc.IdemProcFilePath(idemproc.NEED_RESTART, p.Pid))
		if err != nil {
			log.Fatalf("Error Remove in ProcdMonitor.respawnFailedProcs: %v", err)
		}
	}
}

func (m *ProcdMonitor) Work() {
	go m.waitEvict()
	for {
		m.watchProcds()
		if ok := m.l.TryLock(); !ok {
			continue
		}
		m.markProcsNeedRestart()
		ps := m.getProcsNeedRestart()
		m.respawnProcs(ps)
		m.l.Unlock()
	}
}

func (m *ProcdMonitor) Exit() {
	m.Exited(m.pid)
}

func (m *ProcdMonitor) String() string {
	return fmt.Sprintf("&{ pid:%v }", m.pid)
}
