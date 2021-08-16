package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/fslib"
	//	np "ulambda/ninep"
	"ulambda/sync"
)

type Ttype uint32
type Tcore uint32
type Twait uint32

const (
	T_DEF Ttype = 0
	T_LC  Ttype = 1
	T_BE  Ttype = 2
)

const (
	C_DEF Tcore = 0
)

const (
	START Twait = 0
	EXIT  Twait = 1
)

const (
	START_COND = "start-cond."
	EXIT_COND  = "exit-cond."
)

const (
	RUNQ          = "name/runq"
	RUNQ_PREFIX   = "1_"
	RUNQLC_PREFIX = "0_"
	JOB_SIGNAL    = "job-signal"
	WAIT_START    = "wait-start."
	WAIT_EXIT     = "wait-exit."
	PROC_COND     = "name/proc-cond"
)

// XXX factor out
const (
	NO_OP_LAMBDA = "no-op-lambda"
)

type Proc struct {
	Pid     string   // SigmaOS PID
	Program string   // Program to run
	Dir     string   // Working directory for the process
	Args    []string // Args
	Env     []string // Environment variables
	//	StartDep map[string]bool // Start dependencies // XXX Replace somehow?
	//	ExitDep  map[string]bool // Exit dependencies// XXX Replace somehow?
	Type  Ttype // Type
	Ncore Tcore // Number of cores requested
}

type ProcCtl struct {
	runq *sync.FileBag
	*fslib.FsLib
}

// XXX remove pid arg
func MakeProcCtl(fsl *fslib.FsLib) *ProcCtl {
	pctl := &ProcCtl{}
	pctl.runq = sync.MakeFileBag(fsl, RUNQ)
	pctl.FsLib = fsl

	return pctl
}

// ========== SPAWN ==========

func (pctl *ProcCtl) Spawn(p *Proc) error {
	// Select which queue to put the job in
	var procFName string
	switch p.Type {
	case T_DEF:
		procFName = RUNQ_PREFIX + p.Pid
	case T_LC:
		procFName = RUNQLC_PREFIX + p.Pid
	case T_BE:
		procFName = RUNQ_PREFIX + p.Pid
	default:
		log.Fatalf("Error in ProcCtl.Spawn: Unknown proc type %v", p.Type)
	}

	pStartCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, START_COND+p.Pid), nil)
	pStartCond.Init()

	pExitCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, EXIT_COND+p.Pid), nil)
	pExitCond.Init()

	b, err := json.Marshal(p)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		pStartCond.Destroy()
		pExitCond.Destroy()
		log.Fatalf("Error marshal: %v", err)
		return err
	}

	err = pctl.runq.Put(procFName, b)
	if err != nil {
		log.Printf("Error Put in ProcCtl.Spawn: %v", err)
		return err
	}

	return nil
}

// ========== WAIT ==========

// Wait until a proc has started. If the proc doesn't exist, return immediately.
func (pctl *ProcCtl) WaitStart(pid string) error {
	pStartCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, START_COND+pid), nil)
	pStartCond.Wait()
	return nil
}

// Wait until a proc has exited. If the proc doesn't exist, return immediately.
func (pctl *ProcCtl) WaitExit(pid string) error {
	pExitCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, EXIT_COND+pid), nil)
	pExitCond.Wait()
	return nil
}

// ========== STARTED ==========

// Mark that a process has started.
func (pctl *ProcCtl) Started(pid string) error {
	pStartCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, START_COND+pid), nil)
	pStartCond.Destroy()
	return nil
}

// ========== EXITED ==========

// Mark that a process has exited.
func (pctl *ProcCtl) Exited(pid string) error {
	pExitCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, EXIT_COND+pid), nil)
	pExitCond.Destroy()
	return nil
}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ pid:%v prog:%v dir:%v args:%v env:%v type:%v ncore:%v }", p.Pid, p.Program, p.Dir, p.Args, p.Env, p.Type, p.Ncore)
}
