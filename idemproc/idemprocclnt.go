package idemproc

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
	IDEM_PROCS   = "name/idemprocs"
	UNCLAIMED    = "unclaimed"
	NEED_RESTART = "need-restart"
)

type IdemProcClnt struct {
	proc.ProcClnt
	*fslib.FsLib
}

func MakeIdemProcClnt(fsl *fslib.FsLib, ctl proc.ProcClnt) *IdemProcClnt {
	ictl := &IdemProcClnt{}
	ictl.FsLib = fsl
	ictl.ProcClnt = ctl

	ictl.Init()

	return ictl
}

// ========== NAMING CONVENTIONS ==========

func IdemProcFilePath(procdIP string, pid string) string {
	return path.Join(IDEM_PROCS, procdIP, pid)
}

// ========== INIT ==========

func (ctl *IdemProcClnt) Init() error {
	ctl.Mkdir(IDEM_PROCS, 0777)
	ctl.Mkdir(path.Join(IDEM_PROCS, UNCLAIMED), 0777)
	ctl.Mkdir(path.Join(IDEM_PROCS, NEED_RESTART), 0777)
	return nil
}

// ========== SPAWN ==========

func (ctl *IdemProcClnt) Spawn(gp proc.GenericProc) error {
	p := IdemProc{}
	p.Proc = gp.GetProc()

	idemProcFPath := IdemProcFilePath(UNCLAIMED, p.Pid)

	// Atomically create the idemProc file.
	err := atomic.MakeFileJsonAtomic(ctl.FsLib, idemProcFPath, 0777, p)
	if err != nil {
		return err
	}

	return ctl.ProcClnt.Spawn(p.Proc)
}

// ========== WAIT ==========

// Wait until a proc has started. If the proc doesn't exist, return immediately.
func (ctl *IdemProcClnt) WaitStart(pid string) error {
	return ctl.ProcClnt.WaitStart(pid)
}

// Wait until a proc has exited. If the proc doesn't exist, return immediately.
func (ctl *IdemProcClnt) WaitExit(pid string) error {
	return ctl.ProcClnt.WaitExit(pid)
}

// Wait for a proc's eviction notice. If the proc doesn't exist, return immediately.
func (ctl *IdemProcClnt) WaitEvict(pid string) error {
	return ctl.ProcClnt.WaitEvict(pid)
}

// ========== STARTED ==========

// Mark that a process has started.
func (ctl *IdemProcClnt) Started(pid string) error {
	procdIP := os.Getenv("PROCDIP")
	if len(procdIP) == 0 {
		log.Fatalf("Error: Bad procdIP in IdemProcClnt.Started: %v", procdIP)
		return fmt.Errorf("Error: Bad procdIP in IdemProcClnt.Started: %v", procdIP)
	}
	ctl.Mkdir(path.Join(IDEM_PROCS, procdIP), 0777)
	old := IdemProcFilePath(UNCLAIMED, pid)
	new := IdemProcFilePath(procdIP, pid)
	err := ctl.Rename(old, new)
	if err != nil {
		log.Fatalf("Error: Rename in IdemProcClnt.Started: %v", err)
	}
	return ctl.ProcClnt.Started(pid)
}

// ========== EXITED ==========

// Mark that a process has exited.
func (ctl *IdemProcClnt) Exited(pid string) error {
	procdIP := os.Getenv("PROCDIP")
	if len(procdIP) == 0 {
		log.Fatalf("Error: Bad procdIP in IdemProcClnt.Exited: %v", procdIP)
		return fmt.Errorf("Error: Bad procdIP in IdemProcClnt.Exited: %v", procdIP)
	}
	path := IdemProcFilePath(procdIP, pid)
	err := ctl.Remove(path)
	if err != nil {
		log.Fatalf("Error: Remove in IdemProcClnt.Exited: %v", err)
	}
	return ctl.ProcClnt.Exited(pid)
}

// ========== EVICT ==========

// Notify a process that it will be evicted.
func (ctl *IdemProcClnt) Evict(pid string) error {
	return ctl.ProcClnt.Evict(pid)
}
