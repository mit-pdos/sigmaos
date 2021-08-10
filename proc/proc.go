package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/sync"
)

type Ttype uint32
type Tcore uint32

const (
	T_DEF Ttype = 0
	T_LC  Ttype = 1
	T_BE  Ttype = 2
)

const (
	C_DEF Tcore = 0
)

const (
	RUNQ          = "name/runq"
	RUNQLC        = "name/runqlc"
	WAITQ         = "name/waitq"
	CLAIMED       = "name/claimed"
	CLAIMED_EPH   = "name/claimed_ephemeral"
	SPAWNED       = "name/spawned"
	JOB_SIGNAL    = "job-signal"
	WAIT_LOCK     = "wait-lock."
	CRASH_TIMEOUT = 1
)

const (
	START_DEP = iota
	EXIT_DEP  = iota
)

const (
	WAITFILE_PADDING = 1000
)

type Proc struct {
	Pid      string          // SigmaOS PID
	Program  string          // Program to run
	Dir      string          // Working directory for the process
	Args     []string        // Args
	Env      []string        // Environment variables
	StartDep map[string]bool // Start dependencies // XXX Replace somehow?
	ExitDep  map[string]bool // Exit dependencies// XXX Replace somehow?
	Type     Ttype           // Type
	Ncore    Tcore           // Number of cores requested
	//	Timer    uint32          // Start timer in seconds
	//	WDir       string          // Working directory for the process
	//	StartTimer uint32          // Start timer in seconds
}

type WaitFile struct {
	Started  bool
	StartDep []string // PIDs of lambdas that have a start dependency on this lambda.
	ExitDep  []string // PIDs of lambdas that have a start dependency on this lambda.
}

type ProcCtl struct {
	pid string
	*fslib.FsLib
}

func MakeProcCtl(fsl *fslib.FsLib, pid string) *ProcCtl {
	pctl := &ProcCtl{}
	pctl.FsLib = fsl
	pctl.pid = pid

	return pctl
}

// Notify localds that a job has become runnable
func (pctl *ProcCtl) SignalNewJob() error {
	// Needs to be done twice, since someone waiting on the signal will create a
	// new lock file, even if they've crashed
	return pctl.UnlockFile(fslib.LOCKS, fslib.JOB_SIGNAL)
}

// ========== NAMING CONVENTIONS ==========

func WaitFilePath(pid string) string {
	return path.Join(SPAWNED, waitFileName(pid))
}

func waitFileName(pid string) string {
	return fslib.LockName(WAIT_LOCK + pid)
}

// ========== SPAWN ==========

func (pctl *ProcCtl) Spawn(p *Proc) error {
	// Create a file for waiters to watch & wait on
	err := pctl.makeWaitFile(p.Pid)
	if err != nil {
		return err
	}

	procFPath := path.Join(WAITQ, p.Pid)
	// Create a lock to atomically update the job file.
	pLock := sync.MakeLock(pctl.FsLib, fslib.LOCKS, fslib.LockName(procFPath), true)

	// Lock the job file to make sure we don't miss any dependency updates
	pLock.Lock()
	defer pLock.Unlock()

	// Register dependency backwards pointers.
	pctl.registerDependants(p)

	b, err := json.Marshal(p)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		pctl.removeWaitFile(p.Pid)
		return err
	}

	// Atomically create the job file.
	err = pctl.MakeFileAtomic(procFPath, 0777, b)
	if err != nil {
		return err
	}

	// Start the job if it is runnable
	if pctl.procIsRunnable(p) {
		pctl.runProc(p)
	}

	// Notify localds that a job has joined the queue
	return nil
}

func (pctl *ProcCtl) procIsRunnable(p *Proc) bool {
	// Check for any unexited StartDeps
	for _, started := range p.StartDep {
		if !started {
			return false
		}
	}

	// Check for any unexited ExitDeps
	for _, exited := range p.ExitDep {
		if !exited {
			return false
		}
	}
	return true
}

func (pctl *ProcCtl) runProc(p *Proc) error {
	var err error
	if p.Type == T_LC {
		err = pctl.Rename(path.Join(WAITQ, p.Pid), path.Join(RUNQLC, p.Pid))
	} else {
		err = pctl.Rename(path.Join(WAITQ, p.Pid), path.Join(RUNQ, p.Pid))
	}

	if err != nil {
		log.Fatalf("Error in runProc: %v", err)
	}
	// Notify localds that a job has become runnable
	pctl.SignalNewJob()
	return nil
}

// ========== WAIT ==========

// Wait for a task's completion.
func (pctl *ProcCtl) Wait(pid string) ([]byte, error) {

	// Wait on the lambda with a watch
	done := make(chan bool)
	err := pctl.SetRemoveWatch(WaitFilePath(pid), func(p string, err error) {
		if err != nil && err.Error() == "EOF" {
			return
		} else if err != nil {
			log.Printf("Error in wait watch: %v", err)
		}
		done <- true
	})
	// if error, don't wait; the lambda may already have exited.
	if err == nil {
		<-done
	}

	return []byte{'O', 'K'}, err
}

// ========== STARTED ==========

func (pctl *ProcCtl) Started(pid string) error {
	waitFileLock := sync.MakeLock(pctl.FsLib, fslib.LOCKS, fslib.LockName(WaitFilePath(pid)), true)

	waitFileLock.Lock()
	defer waitFileLock.Unlock()

	pctl.updateDependants(pid, START_DEP)

	return nil
}

func (pctl *ProcCtl) HasStarted(pid string) bool {
	waitFileLock := sync.MakeLock(pctl.FsLib, fslib.LOCKS, fslib.LockName(WaitFilePath(pid)), true)

	waitFileLock.Lock()
	defer waitFileLock.Unlock()

	b, _, err := pctl.GetFile(WaitFilePath(pid))
	if err != nil {
		return false
	}

	var wf WaitFile
	err = json.Unmarshal(b, &wf)
	if err != nil {
		log.Printf("Couldn't unmarshal waitfile in ProcCtl.registerDependant: %v, %v", string(b), err)
	}
	return wf.Started
}

// ========== EXITING ==========

func (pctl *ProcCtl) Exiting(pid string, status string) error {
	waitFileLock := sync.MakeLock(pctl.FsLib, fslib.LOCKS, fslib.LockName(WaitFilePath(pid)), true)

	waitFileLock.Lock()
	defer waitFileLock.Unlock()

	// Update waiters
	pctl.updateDependants(pid, EXIT_DEP)

	err := pctl.Remove(path.Join(CLAIMED, pid))
	if err != nil {
		log.Printf("Error removing claimed in Exiting %v: %v", pid, err)
	}
	err = pctl.Remove(path.Join(CLAIMED_EPH, pid))
	if err != nil {
		log.Printf("Error removing claimed_eph in Exiting %v: %v", pid, err)
	}

	// Release people waiting on this lambda
	return pctl.removeWaitFile(pid)
}

// ========== HELPERS ==========

func (pctl *ProcCtl) makeWaitFile(pid string) error {
	fpath := WaitFilePath(pid)
	var wf WaitFile
	wf.Started = false
	b, err := json.Marshal(wf)
	if err != nil {
		log.Printf("Error marshalling waitfile: %v", err)
	}
	// Make a writable, versioned file
	err = pctl.MakeFile(fpath, 0777, np.OWRITE, b)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() != "EOF" {
		return fmt.Errorf("Error on MakeFile MakeWaitFile %v: %v", fpath, err)
	}
	return nil
}

func (pctl *ProcCtl) removeWaitFile(pid string) error {
	fpath := WaitFilePath(pid)
	err := pctl.Remove(fpath)
	if err != nil {
		log.Printf("Error on RemoveWaitFile  %v: %v", fpath, err)
		return err
	}
	return nil
}

// Register start & exit dependencies in dependencies' waitfiles, and update the
// current proc's dependencies.
func (pctl *ProcCtl) registerDependants(p *Proc) {
	for dep, _ := range p.StartDep {
		if ok := pctl.registerDependant(dep, p.Pid, START_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			p.StartDep[dep] = true
		}
	}
	for dep, _ := range p.ExitDep {
		if ok := pctl.registerDependant(dep, p.Pid, EXIT_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			p.ExitDep[dep] = true
		}
	}
}

// Register a dependency in another Proc's WaitFile. If the registration
// succeeded, return true. If the registration failed, assume the dependency has
// been satisfied, and return false.
func (pctl *ProcCtl) registerDependant(depPid string, waiterPid string, depType int) bool {
	depLock := sync.MakeLock(pctl.FsLib, fslib.LOCKS, fslib.LockName(WaitFilePath(depPid)), true)

	depLock.Lock()
	defer depLock.Unlock()

	b, _, err := pctl.GetFile(WaitFilePath(depPid))
	if err != nil {
		return false
	}

	var wf WaitFile
	err = json.Unmarshal(b, &wf)
	if err != nil {
		log.Printf("Couldn't unmarshal waitfile in ProcCtl.registerDependant: %v, %v", string(b), err)
	}

	switch depType {
	case START_DEP:
		// Check we didn't miss the start signal already.
		if wf.Started {
			return false
		}
		wf.StartDep = append(wf.StartDep, waiterPid)
	case EXIT_DEP:
		wf.ExitDep = append(wf.ExitDep, waiterPid)
	default:
		log.Fatalf("Unknown dep type in ProcCtl.registerDependant: %v", depType)
	}

	// Write back updated deps if
	b2, err := json.Marshal(wf)
	if err != nil {
		log.Fatalf("Error marshalling deps in ProcCtl.registerDependant: %v", err)
	}

	_, err = pctl.SetFile(WaitFilePath(depPid), b2, np.NoV)
	if err != nil {
		log.Printf("Error setting waitfile in ProcCtl.registerDependant: %v, %v", WaitFilePath(depPid), err)
	}

	return true
}

func (pctl *ProcCtl) updateDependants(pid string, depType int) {
	// Get the current contents of the wait file
	b1, _, err := pctl.GetFile(WaitFilePath(pid))
	if err != nil {
		log.Printf("Error GetFile in ProcCtl.updateDependants: %v, %v", WaitFilePath(pid), err)
		return
	}

	// Unmarshal
	var wf WaitFile
	err = json.Unmarshal(b1, &wf)
	if err != nil {
		log.Fatalf("Error unmarshalling waitfile: %v, %v", string(b1), err)
		return
	}

	var waiters []string

	switch depType {
	case START_DEP:
		waiters = wf.StartDep
	case EXIT_DEP:
		waiters = wf.ExitDep
	default:
		log.Fatalf("Unknown depType in ProcCtl.updateDependants: %v", depType)
	}

	for _, waiter := range waiters {
		pctl.updateDependant(pid, waiter, depType)
	}

	// Record the start signal if applicable.
	if depType == START_DEP {
		wf.Started = true
		b2, err := json.Marshal(wf)
		if err != nil {
			log.Printf("Error marshalling waitfile: %v", err)
			return
		}
		b2 = append(b2, ' ')
		_, err = pctl.SetFile(WaitFilePath(pid), b2, np.NoV)
		if err != nil {
			log.Printf("Error SetFile in ProcCtl.updateDependants: %v, %v", WaitFilePath(pid), err)
		}
	}
}

func (pctl *ProcCtl) updateDependant(depPid string, waiterPid string, depType int) {
	waiterFPath := path.Join(WAITQ, waiterPid)

	// Create a lock to atomically update the job file.
	waiterPLock := sync.MakeLock(pctl.FsLib, fslib.LOCKS, fslib.LockName(waiterFPath), true)

	// Lock the job file to make sure we don't miss any dependency updates
	waiterPLock.Lock()
	defer waiterPLock.Unlock()

	b, _, err := pctl.GetFile(waiterFPath)
	if err != nil {
		log.Printf("Couldn't get waiter file in ProcCtl.updateDependant: %v, %v", waiterPid, err)
		return
	}

	p := &Proc{}
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Printf("Couldn't unmarshal job in ProcCtl.updateDependant %v: %v", string(b), err)
	}

	switch depType {
	case START_DEP:
		// If the dependency has already been marked, move along.
		if done := p.StartDep[depPid]; done {
			return
		}
		p.StartDep[depPid] = true
	case EXIT_DEP:
		// If the dependency has already been marked, move along.
		if done := p.ExitDep[depPid]; done {
			return
		}
		p.ExitDep[depPid] = true
	default:
		log.Fatalf("Unknown depType in ProcCtl.updateDependant: %v", depType)
	}

	b2, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling in ProcCtl.updateDependant: %v", err)
	}
	// XXX Hack around lack of OTRUNC
	for i := 0; i < WAITFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = pctl.SetFile(waiterFPath, b2, np.NoV)
	if err != nil {
		log.Printf("Error writing in ProcCtl.updateDependant: %v, %v", waiterFPath, err)
	}

	if pctl.procIsRunnable(p) {
		pctl.runProc(p)
		pctl.SignalNewJob()
	}
}

// XXX REMOVE --- just used by GG
// ========== SPAWN_NO_OP =========

// Spawn a no-op lambda
func (pctl *ProcCtl) SpawnNoOp(pid string, exitDep []string) error {
	a := &Proc{}
	a.Pid = pid
	a.Program = fslib.NO_OP_LAMBDA
	exitDepMap := map[string]bool{}
	for _, dep := range exitDep {
		exitDepMap[dep] = false
	}
	a.ExitDep = exitDepMap
	return pctl.Spawn(a)
}

// ========== SWAP EXIT DEPENDENCY =========

func (pctl *ProcCtl) SwapExitDependency(pids []string) error {
	fromPid := pids[0]
	toPid := pids[1]
	return pctl.modifyExitDependencies(func(deps map[string]bool) bool {
		if _, ok := deps[fromPid]; ok {
			deps[toPid] = false
			deps[fromPid] = true
			return true
		}
		return false
	})
}

func (pctl *ProcCtl) modifyExitDependencies(f func(map[string]bool) bool) error {
	ls, _ := pctl.ReadDir(WAITQ)
	for _, l := range ls {
		// Lock the file
		pctl.LockFile(fslib.LOCKS, path.Join(WAITQ, l.Name))
		a, _, err := pctl.GetFile(path.Join(WAITQ, l.Name))
		// May get file not found if someone renamed the file
		if err != nil && err.Error() != "file not found" {
			pctl.UnlockFile(fslib.LOCKS, path.Join(WAITQ, l.Name))
			continue
		}
		if err != nil {
			log.Fatalf("Error in SwapExitDependency GetFile %v: %v", l.Name, err)
			return err
		}
		var attr Proc
		err = json.Unmarshal(a, &attr)
		if err != nil {
			log.Fatalf("Error in SwapExitDependency Unmarshal %v: %v", a, err)
			return err
		}
		changed := f(attr.ExitDep)
		// If the ExitDep map changed, write it back.
		if changed {
			b, err := json.Marshal(attr)
			if err != nil {
				log.Fatalf("Error in SwapExitDependency Marshal %v: %v", b, err)
				return err
			}
			// XXX OTRUNC isn't implemented for memfs yet, so remove & rewrite
			err = pctl.Remove(path.Join(WAITQ, l.Name))
			// May get file not found if someone renamed the file
			if err != nil && err.Error() != "file not found" {
				pctl.UnlockFile(fslib.LOCKS, path.Join(WAITQ, l.Name))
				continue
			}
			err = pctl.MakeFileAtomic(path.Join(WAITQ, l.Name), 0777, b)
			if err != nil {
				log.Fatalf("Error in SwapExitDependency MakeFileAtomic %v: %v", l.Name, err)
				return err
			}
		}
		pctl.UnlockFile(fslib.LOCKS, path.Join(WAITQ, l.Name))
	}
	return nil
}
