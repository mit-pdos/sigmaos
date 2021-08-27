package baseproc

import (
	"encoding/json"
	"log"
	"os"
	"path"

	"ulambda/fslib"
	"ulambda/namespace"
	"ulambda/proc"
	"ulambda/seccomp"
	"ulambda/sync"
)

type Twait uint32

const (
	START Twait = 0
	EXIT  Twait = 1
)

const (
	START_COND = "start-cond."
	EVICT_COND = "evict-cond."
	EXIT_COND  = "exit-cond."
)

const (
	RUNQLC_PRIORITY = "0"
	RUNQ_PRIORITY   = "1"
)

const (
	RUNQ       = "name/runq"
	JOB_SIGNAL = "job-signal"
	WAIT_START = "wait-start."
	WAIT_EXIT  = "wait-exit."
	PROC_COND  = "name/proc-cond"
)

type BaseProcCtl struct {
	runq *sync.FilePriorityBag
	*fslib.FsLib
}

func MakeBaseProcCtl(fsl *fslib.FsLib) *BaseProcCtl {
	ctl := &BaseProcCtl{}
	ctl.runq = sync.MakeFilePriorityBag(fsl, RUNQ)
	ctl.FsLib = fsl

	return ctl
}

// ========== SPAWN ==========

func (ctl *BaseProcCtl) Spawn(gp proc.GenericProc) error {
	p := gp.GetProc()
	// Select which queue to put the job in
	var procPriority string
	switch p.Type {
	case proc.T_DEF:
		procPriority = RUNQ_PRIORITY
	case proc.T_LC:
		procPriority = RUNQLC_PRIORITY
	case proc.T_BE:
		procPriority = RUNQ_PRIORITY
	default:
		log.Fatalf("Error in BaseProcCtl.Spawn: Unknown proc type %v", p.Type)
	}

	pStartCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, START_COND+p.Pid), nil)
	pStartCond.Init()

	pExitCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, EXIT_COND+p.Pid), nil)
	pExitCond.Init()

	pEvictCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, EVICT_COND+p.Pid), nil)
	pEvictCond.Init()

	b, err := json.Marshal(p)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		pStartCond.Destroy()
		pExitCond.Destroy()
		pEvictCond.Destroy()
		log.Fatalf("Error marshal: %v", err)
		return err
	}

	err = ctl.runq.Put(procPriority, p.Pid, b)
	if err != nil {
		log.Printf("Error Put in BaseProcCtl.Spawn: %v", err)
		return err
	}

	return nil
}

// ========== WAIT ==========

// Wait until a proc has started. If the proc doesn't exist, return immediately.
func (ctl *BaseProcCtl) WaitStart(pid string) error {
	pStartCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, START_COND+pid), nil)
	pStartCond.Wait()
	return nil
}

// Wait until a proc has exited. If the proc doesn't exist, return immediately.
func (ctl *BaseProcCtl) WaitExit(pid string) error {
	pExitCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, EXIT_COND+pid), nil)
	pExitCond.Wait()
	return nil
}

// Wait for a proc's eviction notice. If the proc doesn't exist, return immediately.
func (ctl *BaseProcCtl) WaitEvict(pid string) error {
	pEvictCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, EVICT_COND+pid), nil)
	pEvictCond.Wait()
	return nil
}

// ========== STARTED ==========

// Mark that a process has started.
func (ctl *BaseProcCtl) Started(pid string) error {
	pStartCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, START_COND+pid), nil)
	pStartCond.Destroy()
	// Isolate the process namespace
	newRoot := os.Getenv("NEWROOT")
	if err := namespace.Isolate(newRoot); err != nil {
		log.Fatalf("Error Isolate in ctl.Started: %v", err)
	}
	// Load a seccomp filter.
	seccomp.LoadFilter()
	return nil
}

// ========== EXITED ==========

// Mark that a process has exited.
func (ctl *BaseProcCtl) Exited(pid string) error {
	log.Printf("BaseProcCtl.Exited: %v", pid)
	pExitCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, EXIT_COND+pid), nil)
	pExitCond.Destroy()
	return nil
}

// ========== EVICT ==========

// Notify a process that it will be evicted.
func (ctl *BaseProcCtl) Evict(pid string) error {
	pEvictCond := sync.MakeCond(ctl.FsLib, path.Join(PROC_COND, EVICT_COND+pid), nil)
	pEvictCond.Destroy()
	return nil
}
