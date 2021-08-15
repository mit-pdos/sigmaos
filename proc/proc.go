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
	RUNQ        = "name/runq"
	RUNQLC      = "name/runqlc"
	WAITQ       = "name/waitq"
	CLAIMED     = "name/claimed"
	CLAIMED_EPH = "name/claimed_ephemeral"
	SPAWNED     = "name/spawned"
	JOB_SIGNAL  = "job-signal"
	WAIT_START  = "wait-start."
	WAIT_EXIT   = "wait-exit."
	PROC_COND   = "name/proc-cond"
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
	jobLock *sync.Lock
	*fslib.FsLib
}

// XXX remove pid arg
func MakeProcCtl(fsl *fslib.FsLib) *ProcCtl {
	pctl := &ProcCtl{}
	pctl.FsLib = fsl
	pctl.jobLock = sync.MakeLock(fsl, fslib.LOCKS, JOB_SIGNAL, false)

	return pctl
}

// Notify procds that a job has become runnable
func (pctl *ProcCtl) SignalNewJob() {
	pctl.jobLock.Unlock()
}

// ========== NAMING CONVENTIONS ==========

//func waitFilePath(pid string, waitType Twait) string {
//	var waitFileName string
//	switch waitType {
//	case START:
//		waitFileName = fslib.LockName(WAIT_START + pid)
//	case EXIT:
//		waitFileName = fslib.LockName(WAIT_EXIT + pid)
//	default:
//		log.Fatalf("Unknown wait type: %v", waitType)
//	}
//	return path.Join(SPAWNED, waitFileName)
//}

// ========== SPAWN ==========

func (pctl *ProcCtl) Spawn(p *Proc) error {
	// Select which queue to put the job in
	var procFPath string
	switch p.Type {
	case T_DEF:
		procFPath = path.Join(RUNQ, p.Pid)
	case T_LC:
		procFPath = path.Join(RUNQLC, p.Pid)
	case T_BE:
		procFPath = path.Join(RUNQ, p.Pid)
	default:
		log.Fatalf("Error in ProcCtl.Spawn: Unknown proc type %v", p.Type)
	}

	pStartCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, START_COND+p.Pid), nil)
	pStartCond.Init()

	pExitCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, EXIT_COND+p.Pid), nil)
	pExitCond.Init()

	//	// Create files for waiters to watch & wait on
	//	err := pctl.makeWaitFile(p.Pid, START)
	//	if err != nil {
	//		return err
	//	}

	//	err = pctl.makeWaitFile(p.Pid, EXIT)
	//	if err != nil {
	//		pctl.removeWaitFile(p.Pid, START)
	//		return err
	//	}

	b, err := json.Marshal(p)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		pStartCond.Destroy()
		pExitCond.Destroy()
		log.Fatalf("Error marshal: %v", err)
		//		pctl.removeWaitFile(p.Pid, START)
		//		pctl.removeWaitFile(p.Pid, EXIT)
		return err
	}

	// Atomically create the job file.
	err = pctl.MakeFileAtomic(procFPath, 0777, b)
	if err != nil {
		log.Printf("Error making proc file: %v", err)
		return err
	}

	// Notify procds that a proc has joined the queue.
	pctl.SignalNewJob()

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

// XXX switch to use condvar...
//func (pctl *ProcCtl) wait(pid string, waitType Twait) error {
//	// Wait on the lambda with a watch
//	done := make(chan bool)
//	err := pctl.SetRemoveWatch(waitFilePath(pid, waitType), func(p string, err error) {
//		if err != nil && err.Error() == "EOF" {
//			return
//		} else if err != nil {
//			log.Printf("Error in wait watch: %v", err)
//		}
//		done <- true
//	})
//	// if error, don't wait; the lambda may already have exited.
//	if err == nil {
//		<-done
//	}
//	return err
//}

// ========== STARTED ==========

// Mark that a process has started.
func (pctl *ProcCtl) Started(pid string) error {
	pStartCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, START_COND+pid), nil)
	pStartCond.Destroy()
	//	err := pctl.removeWaitFile(pid, START)
	//	if err != nil {
	//		log.Fatalf("Error Remove in pctl.Started: %v", err)
	//		return err
	//	}
	return nil
}

// ========== EXITED ==========

// Mark that a process has exited.
func (pctl *ProcCtl) Exited(pid string) error {
	pExitCond := sync.MakeCond(pctl.FsLib, path.Join(PROC_COND, EXIT_COND+pid), nil)
	pExitCond.Destroy()
	//	err := pctl.removeWaitFile(pid, EXIT)
	//	if err != nil {
	//		log.Fatalf("Error Remove in pctl.Exited: %v", err)
	//		return err
	//	}
	return nil
}

// ========== JOB MANIPULATION ==========

// XXX Maybe should get rid of this at some point...
func (pctl *ProcCtl) GetProcFile(dir string, pid string) (*Proc, error) {
	fpath := path.Join(dir, pid)

	b, _, err := pctl.GetFile(fpath)
	if err != nil {
		return nil, err
	}

	p := &Proc{}
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Couldn't unmarshal proc file: %v, %v", string(b), err)
		return nil, err
	}
	return p, nil
}

// ========== HELPERS ==========

//func (pctl *ProcCtl) makeWaitFile(pid string, waitType Twait) error {
//	fpath := waitFilePath(pid, waitType)
//	err := pctl.MakeFile(fpath, 0777, np.OWRITE, []byte{})
//	// Sometimes we get "EOF" on shutdown
//	if err != nil && err.Error() != "EOF" {
//		return fmt.Errorf("Error on MakeFile ProcCtl.makeWaitFile %v: %v", fpath, err)
//	}
//	return nil
//}
//
//func (pctl *ProcCtl) removeWaitFile(pid string, waitType Twait) error {
//	fpath := waitFilePath(pid, waitType)
//	err := pctl.Remove(fpath)
//	// Sometimes we get "EOF" on shutdown
//	if err != nil && err.Error() != "EOF" {
//		return fmt.Errorf("Error on RemoveFile ProcCtl.removeWaitFile %v: %v", fpath, err)
//	}
//	return nil
//}

func (p *Proc) String() string {
	return fmt.Sprintf("&{ pid:%v prog:%v dir:%v args:%v env:%v type:%v ncore:%v }", p.Pid, p.Program, p.Dir, p.Args, p.Env, p.Type, p.Ncore)
}
