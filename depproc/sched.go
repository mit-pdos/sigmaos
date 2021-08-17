package depproc

import (
	"encoding/json"
	"log"
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/sync"
)

const (
	DEFAULT_JOB_ID = "default-job-id"
)

const (
	JOBS    = "name/jobs"
	COND    = "cond."
	DEPFILE = "depfile."
)

// XXX REMOVE
const (
	DEPFILE_PADDING = 1000
)

type Tdep uint32

const (
	EXIT_DEP  Tdep = 0
	START_DEP Tdep = 1
)

var usingDepProc = false

type DepProcCtl struct {
	JobID  string
	jobDir string
	pctl   *proc.ProcCtl
	*fslib.FsLib
}

func MakeJob(fsl *fslib.FsLib, jid string) {
	// Make sure someone created the jobs dir
	fsl.Mkdir(JOBS, 0777)

	// Make a directory in which to store task info
	err := fsl.Mkdir(path.Join(JOBS, jid), 0777)
	if err != nil {
		db.DLPrintf("DEPPROC", "Error creating job dir: %v", err)
	}
}

func MakeDepProcCtl(fsl *fslib.FsLib, jid string) *DepProcCtl {
	sched := &DepProcCtl{}
	sched.JobID = jid
	sched.pctl = proc.MakeProcCtl(fsl)
	sched.FsLib = fsl
	sched.jobDir = path.Join(JOBS, jid)

	MakeJob(fsl, DEFAULT_JOB_ID)
	usingDepProc = true

	return sched
}

// ========== NAMING CONVENTIONS ==========

func (sched *DepProcCtl) taskFilePath(pid string) string {
	return path.Join(sched.jobDir, pid)
}

func (sched *DepProcCtl) depFilePath(pid string) string {
	return path.Join(sched.jobDir, DEPFILE+pid)
}

// ========== SPAWN ==========

func (sched *DepProcCtl) Spawn(t *Task) error {
	taskFPath := path.Join(sched.jobDir, t.Pid)

	// If the underlying proc hasn't been spawned yet, the Waits will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(sched.FsLib, path.Join(sched.jobDir, COND+t.Pid), nil)
	tSpawnCond.Init()

	// Create a lock to make sure we don't miss updates from tasks we depend on.
	tLock := sync.MakeLock(sched.FsLib, fslib.LOCKS, fslib.LockName(taskFPath), true)

	// Lock the task file to make sure we don't miss any dependency updates.
	tLock.Lock()
	defer tLock.Unlock()

	// Register dependency backwards pointers.
	sched.registerDependencies(t)

	b, err := json.Marshal(t)
	if err != nil {
		// Release waiters if spawn fails.
		tSpawnCond.Destroy()
		return err
	}

	// Atomically create the task file.
	err = sched.MakeFileAtomic(taskFPath, 0777, b)
	if err != nil {
		return err
	}

	// Start the task if it is runnable
	if sched.taskIsRunnable(t) {
		sched.runTask(t)
	}

	return nil
}

// ========== WAIT ==========

// Wait for a task to start
func (sched *DepProcCtl) WaitStart(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitStart will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(sched.FsLib, path.Join(sched.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return sched.pctl.WaitStart(pid)
}

// Wait for a task to exit
func (sched *DepProcCtl) WaitExit(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitExit will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(sched.FsLib, path.Join(sched.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return sched.pctl.WaitExit(pid)
}

// ========== STARTED ==========

func (sched *DepProcCtl) Started(pid string) error {
	// Lock the task file
	l := sync.MakeLock(sched.FsLib, fslib.LOCKS, fslib.LockName(sched.taskFilePath(pid)), true)

	l.Lock()
	defer l.Unlock()

	// Update tasks that depend on this task.
	sched.updateDependants(pid, START_DEP)
	sched.pctl.Started(pid)

	return nil
}

// ========== EXITED ==========

func (sched *DepProcCtl) Exited(pid string) error {
	// Lock the task file
	l := sync.MakeLock(sched.FsLib, fslib.LOCKS, fslib.LockName(sched.taskFilePath(pid)), true)

	l.Lock()
	defer l.Unlock()

	// Update tasks that depend on this task.
	sched.updateDependants(pid, EXIT_DEP)
	sched.pctl.Exited(pid)

	err := sched.Remove(sched.taskFilePath(pid))
	if err != nil {
		if usingDepProc {
			db.DLPrintf("DEPPROC", "Error removing task file in DepProcCtl.Exited: %v", err)
		} else {
			log.Printf("Error removing task file in DepProcCtl.Exited: %v", err)
		}
		return err
	}

	return nil
}

// ========== HELPERS ==========

func (sched *DepProcCtl) taskIsRunnable(t *Task) bool {
	// Check for any unexited StartDeps
	for _, started := range t.Dependencies.StartDep {
		if !started {
			return false
		}
	}

	// Check for any unexited ExitDeps
	for _, exited := range t.Dependencies.ExitDep {
		if !exited {
			return false
		}
	}
	return true
}

func (sched *DepProcCtl) runTask(t *Task) {
	err := sched.pctl.Spawn(t.Proc)
	if err != nil {
		log.Fatalf("Error spawning task in DepProcCtl.runTask: %v", err)
	}
	// Release waiters and allow them to wait on the underlying proc.
	tSpawnCond := sync.MakeCond(sched.FsLib, path.Join(sched.jobDir, COND+t.Pid), nil)
	tSpawnCond.Destroy()
}

func (sched *DepProcCtl) getTask(pid string) (*Task, error) {
	b, _, err := sched.GetFile(sched.taskFilePath(pid))
	if err != nil {
		return nil, err
	}

	t := MakeTask()
	err = json.Unmarshal(b, t)
	if err != nil {
		log.Fatalf("Couldn't unmarshal waitfile: %v, %v", string(b), err)
		return nil, err
	}
	return t, nil
}

// Register start & exit dependencies in dependencies' waitfiles, and update the
// current proc's dependencies.
func (sched *DepProcCtl) registerDependencies(t *Task) {
	for dep, _ := range t.Dependencies.StartDep {
		if ok := sched.registerDependant(dep, t.Pid, START_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			t.Dependencies.StartDep[dep] = true
		}
	}
	for dep, _ := range t.Dependencies.ExitDep {
		if ok := sched.registerDependant(dep, t.Pid, EXIT_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			t.Dependencies.ExitDep[dep] = true
		}
	}
}

// Register a dependency on another the Task corresponding to pid. If the
// registration succeeded, return true. If the registration failed, assume the
// dependency has been satisfied, and return false.
func (sched *DepProcCtl) registerDependant(pid string, dependant string, depType Tdep) bool {
	l := sync.MakeLock(sched.FsLib, fslib.LOCKS, fslib.LockName(sched.taskFilePath(pid)), true)

	l.Lock()
	defer l.Unlock()

	t, err := sched.getTask(pid)
	if err != nil {
		return false
	}

	switch depType {
	case START_DEP:
		// Check we didn't miss the start signal already.
		if t.Started {
			return false
		}
		t.Dependants.StartDep[dependant] = false
	case EXIT_DEP:
		t.Dependants.ExitDep[dependant] = false
	default:
		log.Fatalf("Unknown dep type in DepProcCtl.registerDependant: %v", depType)
	}

	// Write back updated deps
	b2, err := json.Marshal(t)
	if err != nil {
		log.Fatalf("Error marshalling deps in ProcCtl.registerDependant: %v", err)
	}

	_, err = sched.SetFile(sched.taskFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error setting waitfile in ProcCtl.registerDependant: %v, %v", sched.taskFilePath(pid), err)
	}

	return true
}

// Update dependants of the Task named by pid.
func (sched *DepProcCtl) updateDependants(pid string, depType Tdep) {
	// Get the current contents of the wait file
	t, err := sched.getTask(pid)
	if err != nil {
		db.DLPrintf("SCHEDCTL", "Error GetFile in DepProcCtl.updateDependants: %v, %v", sched.taskFilePath(pid), err)
		return
	}

	var dependants map[string]bool

	switch depType {
	case START_DEP:
		dependants = t.Dependants.StartDep
	case EXIT_DEP:
		dependants = t.Dependants.ExitDep
	default:
		log.Fatalf("Unknown depType in DepProcCtl.updateDependants: %v", depType)
	}

	for dependant, _ := range dependants {
		sched.updateDependant(pid, dependant, depType)
	}

	// Record the start signal if applicable.
	if depType == START_DEP {
		t.Started = true
		b2, err := json.Marshal(t)
		if err != nil {
			log.Printf("Error marshalling taskfile: %v", err)
			return
		}
		b2 = append(b2, ' ')
		_, err = sched.SetFile(sched.taskFilePath(pid), b2, np.NoV)
		if err != nil {
			log.Printf("Error SetFile in DepProcCtl.updateDependants: %v, %v", sched.taskFilePath(pid), err)
		}
	}
}

// Update the dependency pid of dependant.
func (sched *DepProcCtl) updateDependant(pid string, dependant string, depType Tdep) {
	// Create a lock to atomically update the job file.
	l := sync.MakeLock(sched.FsLib, fslib.LOCKS, fslib.LockName(sched.taskFilePath(dependant)), true)

	// Lock the job file to make sure we don't miss any dependency updates
	l.Lock()
	defer l.Unlock()

	t, err := sched.getTask(dependant)
	if err != nil {
		log.Printf("Couldn't get waiter file in DepProcCtl.updateDependant: %v, %v", dependant, err)
		return
	}

	switch depType {
	case START_DEP:
		// If the dependency has already been marked, move along.
		if done := t.Dependencies.StartDep[pid]; done {
			return
		}
		t.Dependencies.StartDep[pid] = true
	case EXIT_DEP:
		// If the dependency has already been marked, move along.
		if done := t.Dependencies.ExitDep[pid]; done {
			return
		}
		t.Dependencies.ExitDep[pid] = true
	default:
		log.Fatalf("Unknown depType in DepProcCtl.updateDependant: %v", depType)
	}

	b2, err := json.Marshal(t)
	if err != nil {
		log.Fatalf("Error marshalling in DepProcCtl.updateDependant: %v", err)
	}
	// XXX Hack around lack of OTRUNC
	for i := 0; i < DEPFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = sched.SetFile(sched.taskFilePath(dependant), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing in ProcCtl.updateDependant: %v, %v", sched.taskFilePath(dependant), err)
	}

	if sched.taskIsRunnable(t) {
		sched.runTask(t)
	}
}

// XXX REMOVE
func (sched *DepProcCtl) SpawnNoOp(pid string, extiDep []string) error {
	log.Fatalf("SpawnNoOp not implemented")
	return nil
}

func (sched *DepProcCtl) SwapExitDependency(pids []string) error {
	log.Fatalf("SwapExitDependency not implemented")
	return nil
}
