package procidem

import (
	"fmt"
	"log"
	"os"
	"path"

	"ulambda/atomic"
	"ulambda/fslib"
	"ulambda/proc"
)

const (
	IDEM_PROCS   = "name/procidems"
	UNCLAIMED    = "unclaimed"
	NEED_RESTART = "need-restart"
)

type ProcIdemClnt struct {
	proc.ProcClnt
	*fslib.FsLib
}

func MakeProcIdemClnt(fsl *fslib.FsLib, clnt proc.ProcClnt) *ProcIdemClnt {
	iclnt := &ProcIdemClnt{}
	iclnt.FsLib = fsl
	iclnt.ProcClnt = clnt

	iclnt.Init()

	return iclnt
}

// ========== NAMING CONVENTIONS ==========

func ProcIdemFilePath(procdIP string, pid string) string {
	return path.Join(IDEM_PROCS, procdIP, pid)
}

// ========== INIT ==========

func (clnt *ProcIdemClnt) Init() error {
	clnt.Mkdir(IDEM_PROCS, 0777)
	clnt.Mkdir(path.Join(IDEM_PROCS, UNCLAIMED), 0777)
	clnt.Mkdir(path.Join(IDEM_PROCS, NEED_RESTART), 0777)
	return nil
}

// ========== SPAWN ==========

func (clnt *ProcIdemClnt) Spawn(gp proc.GenericProc) error {
	p := ProcIdem{}
	p.Proc = gp.GetProc()

	procIdemFPath := ProcIdemFilePath(UNCLAIMED, p.Pid)

	// Atomically create the procIdem file.
	err := atomic.MakeFileJsonAtomic(clnt.FsLib, procIdemFPath, 0777, p)
	if err != nil {
		return err
	}

	return clnt.ProcClnt.Spawn(p.Proc)
}

// ========== WAIT ==========

// Wait until a proc has started. If the proc doesn't exist, return immediately.
func (clnt *ProcIdemClnt) WaitStart(pid string) error {
	return clnt.ProcClnt.WaitStart(pid)
}

// Wait until a proc has exited. If the proc doesn't exist, return immediately.
func (clnt *ProcIdemClnt) WaitExit(pid string) (string, error) {
	return clnt.ProcClnt.WaitExit(pid)
}

// Wait for a proc's eviction notice. If the proc doesn't exist, return immediately.
func (clnt *ProcIdemClnt) WaitEvict(pid string) error {
	return clnt.ProcClnt.WaitEvict(pid)
}

// ========== STARTED ==========

// Mark that a process has started.
func (clnt *ProcIdemClnt) Started(pid string) error {
	procdIP := os.Getenv("PROCDIP")
	if len(procdIP) == 0 {
		log.Fatalf("Error: Bad procdIP in ProcIdemClnt.Started: %v", procdIP)
		return fmt.Errorf("Error: Bad procdIP in ProcIdemClnt.Started: %v", procdIP)
	}
	clnt.Mkdir(path.Join(IDEM_PROCS, procdIP), 0777)
	old := ProcIdemFilePath(UNCLAIMED, pid)
	new := ProcIdemFilePath(procdIP, pid)
	err := clnt.Rename(old, new)
	if err != nil {
		log.Fatalf("Error: Rename in ProcIdemClnt.Started: %v", err)
	}
	return clnt.ProcClnt.Started(pid)
}

// ========== EXITED ==========

// Mark that a process has exited.
func (clnt *ProcIdemClnt) Exited(pid string, status string) error {
	procdIP := os.Getenv("PROCDIP")
	if len(procdIP) == 0 {
		log.Fatalf("Error: Bad procdIP in ProcIdemClnt.Exited: %v", procdIP)
		return fmt.Errorf("Error: Bad procdIP in ProcIdemClnt.Exited: %v", procdIP)
	}
	path := ProcIdemFilePath(procdIP, pid)
	err := clnt.Remove(path)
	if err != nil {
		log.Fatalf("Error: Remove in ProcIdemClnt.Exited: %v", err)
	}
	return clnt.ProcClnt.Exited(pid, status)
}

// ========== EVICT ==========

// Notify a process that it will be evicted.
func (clnt *ProcIdemClnt) Evict(pid string) error {
	return clnt.ProcClnt.Evict(pid)
}
