package idemproc

import (
	"ulambda/fslib"
	"ulambda/proc"
	//	"ulambda/sync"
)

type IdemProcCtl struct {
	*proc.ProcCtl
	*fslib.FsLib
}

func MakeIdemProcCtl(fsl *fslib.FsLib) *IdemProcCtl {
	ctl := &IdemProcCtl{}
	ctl.FsLib = fsl
	ctl.ProcCtl = proc.MakeProcCtl(fsl)

	return ctl
}

// ========== SPAWN ==========

func (ctl *IdemProcCtl) Spawn(p *IdemProc) error {
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
	return ctl.ProcCtl.Started(pid)
}

// ========== EXITED ==========

// Mark that a process has exited.
func (ctl *IdemProcCtl) Exited(pid string) error {
	return ctl.ProcCtl.Exited(pid)
}

// ========== EVICT ==========

// Notify a process that it will be evicted.
func (ctl *IdemProcCtl) Evict(pid string) error {
	return ctl.ProcCtl.Evict(pid)
}
