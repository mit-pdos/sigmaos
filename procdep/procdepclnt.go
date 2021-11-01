package procdep

import (
	"encoding/json"
	"log"
	"path"

	"ulambda/atomic"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
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

type Tdep uint32

const (
	EXIT_DEP  Tdep = 0
	START_DEP Tdep = 1
)

var usingProcDep = false

type ProcDepClnt struct {
	JobID  string
	jobDir string
	proc.ProcClnt
	*fslib.FsLib
}

func MakeJob(fsl *fslib.FsLib, jid string) {
	// Make sure someone created the jobs dir
	fsl.Mkdir(JOBS, 0777)

	// Make a directory in which to store procDep info
	err := fsl.Mkdir(path.Join(JOBS, jid), 0777)
	if err != nil {
		db.DLPrintf("PROCDEP", "Error creating job dir: %v", err)
	}
}

func MakeProcDepClnt(fsl *fslib.FsLib, clnt proc.ProcClnt) *ProcDepClnt {
	jid := DEFAULT_JOB_ID
	dclnt := &ProcDepClnt{}
	dclnt.JobID = jid
	dclnt.ProcClnt = clnt
	dclnt.FsLib = fsl
	dclnt.jobDir = path.Join(JOBS, jid)

	MakeJob(fsl, DEFAULT_JOB_ID)
	usingProcDep = true

	return dclnt
}

// ========== NAMING CONVENTIONS ==========

func (clnt *ProcDepClnt) procDepFilePath(pid string) string {
	return path.Join(clnt.jobDir, pid)
}

func (clnt *ProcDepClnt) depFilePath(pid string) string {
	return path.Join(clnt.jobDir, DEPFILE+pid)
}

// ========== SPAWN ==========

func (clnt *ProcDepClnt) Spawn(gp proc.GenericProc) error {
	var p *ProcDep
	switch gp.(type) {
	case *ProcDep:
		p = gp.(*ProcDep)
	case *proc.Proc:
		p = MakeEmptyProcDep()
		p.Proc = gp.(*proc.Proc)
	}
	procDepFPath := path.Join(clnt.jobDir, p.Pid)

	// If the underlying proc hasn't been spawned yet, the Waits will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+p.Pid), nil)
	tSpawnCond.Init()

	// Create a lock to make sure we don't miss updates from procDeps we depend on.
	tLock := sync.MakeLock(clnt.FsLib, named.LOCKS, procDepFPath, true)

	// Lock the procDep file to make sure we don't miss any dependency updates.
	tLock.Lock()
	defer tLock.Unlock()

	// Register dependency backwards pointers.
	clnt.registerDependencies(p)

	// Atomically create the procDep file.
	err := atomic.MakeFileJsonAtomic(clnt.FsLib, procDepFPath, 0777, p)
	if err != nil {
		// Release waiters if spawn fails.
		tSpawnCond.Destroy()
		return err
	}

	// Start the procDep if it is runnable
	if clnt.procDepIsRunnable(p) {
		clnt.runProcDep(p)
	}

	return nil
}

// ========== WAIT ==========

// Wait for a procDep to start
func (clnt *ProcDepClnt) WaitStart(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitStart will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return clnt.ProcClnt.WaitStart(pid)
}

// Wait for a procDep to exit
func (clnt *ProcDepClnt) WaitExit(pid string) (string, error) {
	// If the underlying proc hasn't been spawned yet, the WaitExit will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return clnt.ProcClnt.WaitExit(pid)
}

// Wait for a procDep to evict
func (clnt *ProcDepClnt) WaitEvict(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitEvict will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return clnt.ProcClnt.WaitEvict(pid)
}

// ========== STARTED ==========

func (clnt *ProcDepClnt) Started(pid string) error {
	// Lock the procDep file
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.procDepFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	// Update procDeps that depend on this procDep.
	clnt.updateDependants(pid, START_DEP)
	clnt.ProcClnt.Started(pid)

	return nil
}

// ========== EXITED ==========

func (clnt *ProcDepClnt) Exited(pid string, status string) error {
	// Lock the procDep file
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.procDepFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	// Update procDeps that depend on this procDep.
	clnt.updateDependants(pid, EXIT_DEP)
	clnt.ProcClnt.Exited(pid, status)

	err := clnt.Remove(clnt.procDepFilePath(pid))
	if err != nil {
		if usingProcDep {
			db.DLPrintf("PROCDEP", "Error removing procDep file in ProcDepClnt.Exited: %v", err)
		} else {
			log.Printf("Error removing procDep file in ProcDepClnt.Exited: %v", err)
		}
		return err
	}

	return nil
}

// ========== EVICTED ==========

func (clnt *ProcDepClnt) Evict(pid string) error {
	return clnt.ProcClnt.Evict(pid)
}

// ========== HELPERS ==========

func (clnt *ProcDepClnt) procDepIsRunnable(p *ProcDep) bool {
	// Check for any unexited StartDeps
	for _, started := range p.Dependencies.StartDep {
		if !started {
			return false
		}
	}

	// Check for any unexited ExitDeps
	for _, exited := range p.Dependencies.ExitDep {
		if !exited {
			return false
		}
	}
	return true
}

func (clnt *ProcDepClnt) runProcDep(p *ProcDep) {
	err := clnt.ProcClnt.Spawn(p.Proc)
	if err != nil {
		log.Fatalf("Error spawning procDep in ProcDepClnt.runProcDep: %v", err)
	}
	// Release waiters and allow them to wait on the underlying proc.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+p.Pid), nil)
	tSpawnCond.Destroy()
}

func (clnt *ProcDepClnt) getProcDep(pid string) (*ProcDep, error) {
	b, _, err := clnt.GetFile(clnt.procDepFilePath(pid))
	if err != nil {
		return nil, err
	}

	p := MakeEmptyProcDep()
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Couldn't unmarshal waitfile: %v, %v", string(b), err)
		return nil, err
	}
	return p, nil
}

// Register start & exit dependencies in dependencies' waitfiles, and update the
// current proc's dependencies.
func (clnt *ProcDepClnt) registerDependencies(p *ProcDep) {
	for dep, _ := range p.Dependencies.StartDep {
		if ok := clnt.registerDependant(dep, p.Pid, START_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			p.Dependencies.StartDep[dep] = true
		}
	}
	for dep, _ := range p.Dependencies.ExitDep {
		if ok := clnt.registerDependant(dep, p.Pid, EXIT_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			p.Dependencies.ExitDep[dep] = true
		}
	}
}

// Register a dependency on another the ProcDep corresponding to pid. If the
// registration succeeded, return true. If the registration failed, assume the
// dependency has been satisfied, and return false.
func (clnt *ProcDepClnt) registerDependant(pid string, dependant string, depType Tdep) bool {
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.procDepFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	p, err := clnt.getProcDep(pid)
	if err != nil {
		return false
	}

	switch depType {
	case START_DEP:
		// Check we didn't miss the start signal already.
		if p.Started {
			return false
		}
		p.Dependants.StartDep[dependant] = false
	case EXIT_DEP:
		p.Dependants.ExitDep[dependant] = false
	default:
		log.Fatalf("Unknown dep type in ProcDepClnt.registerDependant: %v", depType)
	}

	// Write back updated deps
	b2, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling deps in ProcClnt.registerDependant: %v", err)
	}

	_, err = clnt.SetFile(clnt.procDepFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error setting waitfile in ProcClnt.registerDependant: %v, %v", clnt.procDepFilePath(pid), err)
	}

	return true
}

// Update dependants of the ProcDep named by pid.
func (clnt *ProcDepClnt) updateDependants(pid string, depType Tdep) {
	// Get the current contents of the wait file
	p, err := clnt.getProcDep(pid)
	if err != nil {
		db.DLPrintf("SCHEDCTL", "Error GetFile in ProcDepClnt.updateDependants: %v, %v", clnt.procDepFilePath(pid), err)
		return
	}

	var dependants map[string]bool

	switch depType {
	case START_DEP:
		dependants = p.Dependants.StartDep
	case EXIT_DEP:
		dependants = p.Dependants.ExitDep
	default:
		log.Fatalf("Unknown depType in ProcDepClnt.updateDependants: %v", depType)
	}

	for dependant, _ := range dependants {
		clnt.updateDependant(pid, dependant, depType)
	}

	// Record the start signal if applicable.
	if depType == START_DEP {
		p.Started = true
		err := atomic.MakeFileJsonAtomic(clnt.FsLib, clnt.procDepFilePath(pid), 0777, p)
		if err != nil {
			log.Printf("Error MakeFileJsonAtomic in ProcDepClnt.updateDependants: %v, %v", clnt.procDepFilePath(pid), err)
		}
	}
}

// Update the dependency pid of dependant.
func (clnt *ProcDepClnt) updateDependant(pid string, dependant string, depType Tdep) {
	// Create a lock to atomically update the job file.
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.procDepFilePath(dependant), true)

	// Lock the job file to make sure we don't miss any dependency updates
	l.Lock()
	defer l.Unlock()

	p, err := clnt.getProcDep(dependant)
	if err != nil {
		log.Printf("Couldn't get waiter file in ProcDepClnt.updateDependant: %v, %v", dependant, err)
		return
	}

	switch depType {
	case START_DEP:
		// If the dependency has already been marked, move along.
		if done := p.Dependencies.StartDep[pid]; done {
			return
		}
		p.Dependencies.StartDep[pid] = true
	case EXIT_DEP:
		// If the dependency has already been marked, move along.
		if done := p.Dependencies.ExitDep[pid]; done {
			return
		}
		p.Dependencies.ExitDep[pid] = true
	default:
		log.Fatalf("Unknown depType in ProcDepClnt.updateDependant: %v", depType)
	}

	// XXX Might have too many RPCs
	err = atomic.MakeFileJsonAtomic(clnt.FsLib, clnt.procDepFilePath(dependant), 0777, p)
	if err != nil {
		log.Fatalf("Error MakeFileJsonAtomic in ProcDepClnt.updateDependant: %v", err)
	}

	if clnt.procDepIsRunnable(p) {
		clnt.runProcDep(p)
	}
}

// XXX REMOVE
func (clnt *ProcDepClnt) SpawnNoOp(pid string, extiDep []string) error {
	log.Fatalf("SpawnNoOp not implemented")
	return nil
}

func (clnt *ProcDepClnt) SwapExitDependency(pids []string) error {
	log.Fatalf("SwapExitDependency not implemented")
	return nil
}
