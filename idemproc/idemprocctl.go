package idemproc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"

	"ulambda/fslib"
	"ulambda/proc"
	//	"ulambda/sync"
)

const (
	IDEM_PROCS   = "name/idemprocs"
	UNCLAIMED    = "unclaimed"
	NEED_RESTART = "need-restart"
)

type IdemProcCtl struct {
	proc.ProcCtl
	*fslib.FsLib
}

func MakeIdemProcCtl(fsl *fslib.FsLib, ctl proc.ProcCtl) *IdemProcCtl {
	ictl := &IdemProcCtl{}
	ictl.FsLib = fsl
	ictl.ProcCtl = ctl

	ictl.Init()

	return ictl
}

// ========== NAMING CONVENTIONS ==========

func IdemProcFilePath(procdIP string, pid string) string {
	return path.Join(IDEM_PROCS, procdIP, pid)
}

// ========== INIT ==========

func (ctl *IdemProcCtl) Init() error {
	ctl.Mkdir(IDEM_PROCS, 0777)
	ctl.Mkdir(path.Join(IDEM_PROCS, UNCLAIMED), 0777)
	ctl.Mkdir(path.Join(IDEM_PROCS, NEED_RESTART), 0777)
	return nil
}

// ========== SPAWN ==========

func (ctl *IdemProcCtl) Spawn(gp proc.GenericProc) error {
	p := IdemProc{}
	p.Proc = gp.GetProc()
	b, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling IdemProc in IdemProcCtl.Spawn: %v", err)
		return err
	}

	idemProcFPath := IdemProcFilePath(UNCLAIMED, p.Pid)

	// Atomically create the idemProc file.
	err = ctl.MakeFileAtomic(idemProcFPath, 0777, b)
	if err != nil {
		return err
	}

	return ctl.ProcCtl.Spawn(p.Proc)
}

// ========== WAIT ==========

// Wait until a proc has started. If the proc doesn't exist, return immediately.
func (ctl *IdemProcCtl) WaitStart(pid string) error {
	return ctl.ProcCtl.WaitStart(pid)
}

// Wait until a proc has exited. If the proc doesn't exist, return immediately.
func (ctl *IdemProcCtl) WaitExit(pid string) error {
	return ctl.ProcCtl.WaitExit(pid)
}

// Wait for a proc's eviction notice. If the proc doesn't exist, return immediately.
func (ctl *IdemProcCtl) WaitEvict(pid string) error {
	return ctl.ProcCtl.WaitEvict(pid)
}

// ========== STARTED ==========

// Mark that a process has started.
func (ctl *IdemProcCtl) Started(pid string) error {
	procdIP := os.Getenv("PROCDIP")
	if len(procdIP) == 0 {
		log.Fatalf("Error: Bad procdIP in IdemProcCtl.Started: %v", procdIP)
		return fmt.Errorf("Error: Bad procdIP in IdemProcCtl.Started: %v", procdIP)
	}
	ctl.Mkdir(path.Join(IDEM_PROCS, procdIP), 0777)
	old := IdemProcFilePath(UNCLAIMED, pid)
	new := IdemProcFilePath(procdIP, pid)
	err := ctl.Rename(old, new)
	if err != nil {
		log.Fatalf("Error: Rename in IdemProcCtl.Started: %v", err)
	}
	return ctl.ProcCtl.Started(pid)
}

// ========== EXITED ==========

// Mark that a process has exited.
func (ctl *IdemProcCtl) Exited(pid string) error {
	procdIP := os.Getenv("PROCDIP")
	if len(procdIP) == 0 {
		log.Fatalf("Error: Bad procdIP in IdemProcCtl.Exited: %v", procdIP)
		return fmt.Errorf("Error: Bad procdIP in IdemProcCtl.Exited: %v", procdIP)
	}
	path := IdemProcFilePath(procdIP, pid)
	err := ctl.Remove(path)
	if err != nil {
		log.Fatalf("Error: Remove in IdemProcCtl.Exited: %v", err)
	}
	return ctl.ProcCtl.Exited(pid)
}

// ========== EVICT ==========

// Notify a process that it will be evicted.
func (ctl *IdemProcCtl) Evict(pid string) error {
	return ctl.ProcCtl.Evict(pid)
}
