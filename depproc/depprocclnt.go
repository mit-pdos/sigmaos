package depproc

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

type DepProcClnt struct {
	JobID  string
	jobDir string
	proc.ProcClnt
	*fslib.FsLib
}

func MakeJob(fsl *fslib.FsLib, jid string) {
	// Make sure someone created the jobs dir
	fsl.Mkdir(JOBS, 0777)

	// Make a directory in which to store depProc info
	err := fsl.Mkdir(path.Join(JOBS, jid), 0777)
	if err != nil {
		db.DLPrintf("DEPPROC", "Error creating job dir: %v", err)
	}
}

func MakeDepProcClnt(fsl *fslib.FsLib, clnt proc.ProcClnt) *DepProcClnt {
	jid := DEFAULT_JOB_ID
	dclnt := &DepProcClnt{}
	dclnt.JobID = jid
	dclnt.ProcClnt = clnt
	dclnt.FsLib = fsl
	dclnt.jobDir = path.Join(JOBS, jid)

	MakeJob(fsl, DEFAULT_JOB_ID)
	usingDepProc = true

	return dclnt
}

// ========== NAMING CONVENTIONS ==========

func (clnt *DepProcClnt) depProcFilePath(pid string) string {
	return path.Join(clnt.jobDir, pid)
}

func (clnt *DepProcClnt) depFilePath(pid string) string {
	return path.Join(clnt.jobDir, DEPFILE+pid)
}

// ========== SPAWN ==========

func (clnt *DepProcClnt) Spawn(gp proc.GenericProc) error {
	var p *DepProc
	switch gp.(type) {
	case *DepProc:
		p = gp.(*DepProc)
	case *proc.Proc:
		p = MakeDepProc()
		p.Proc = gp.(*proc.Proc)
	}
	depProcFPath := path.Join(clnt.jobDir, p.Pid)

	// If the underlying proc hasn't been spawned yet, the Waits will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+p.Pid), nil)
	tSpawnCond.Init()

	// Create a lock to make sure we don't miss updates from depProcs we depend on.
	tLock := sync.MakeLock(clnt.FsLib, named.LOCKS, depProcFPath, true)

	// Lock the depProc file to make sure we don't miss any dependency updates.
	tLock.Lock()
	defer tLock.Unlock()

	// Register dependency backwards pointers.
	clnt.registerDependencies(p)

	// Atomically create the depProc file.
	err := atomic.MakeFileJsonAtomic(clnt.FsLib, depProcFPath, 0777, p)
	if err != nil {
		// Release waiters if spawn fails.
		tSpawnCond.Destroy()
		return err
	}

	// Start the depProc if it is runnable
	if clnt.depProcIsRunnable(p) {
		clnt.runDepProc(p)
	}

	return nil
}

// ========== WAIT ==========

// Wait for a depProc to start
func (clnt *DepProcClnt) WaitStart(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitStart will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return clnt.ProcClnt.WaitStart(pid)
}

// Wait for a depProc to exit
func (clnt *DepProcClnt) WaitExit(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitExit will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return clnt.ProcClnt.WaitExit(pid)
}

// Wait for a depProc to evict
func (clnt *DepProcClnt) WaitEvict(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitEvict will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return clnt.ProcClnt.WaitEvict(pid)
}

// ========== STARTED ==========

func (clnt *DepProcClnt) Started(pid string) error {
	// Lock the depProc file
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.depProcFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	// Update depProcs that depend on this depProc.
	clnt.updateDependants(pid, START_DEP)
	clnt.ProcClnt.Started(pid)

	return nil
}

// ========== EXITED ==========

func (clnt *DepProcClnt) Exited(pid string) error {
	// Lock the depProc file
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.depProcFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	// Update depProcs that depend on this depProc.
	clnt.updateDependants(pid, EXIT_DEP)
	clnt.ProcClnt.Exited(pid)

	err := clnt.Remove(clnt.depProcFilePath(pid))
	if err != nil {
		if usingDepProc {
			db.DLPrintf("DEPPROC", "Error removing depProc file in DepProcClnt.Exited: %v", err)
		} else {
			log.Printf("Error removing depProc file in DepProcClnt.Exited: %v", err)
		}
		return err
	}

	return nil
}

// ========== EVICTED ==========

func (clnt *DepProcClnt) Evict(pid string) error {
	return clnt.ProcClnt.Evict(pid)
}

// ========== HELPERS ==========

func (clnt *DepProcClnt) depProcIsRunnable(p *DepProc) bool {
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

func (clnt *DepProcClnt) runDepProc(p *DepProc) {
	err := clnt.ProcClnt.Spawn(p.Proc)
	if err != nil {
		log.Fatalf("Error spawning depProc in DepProcClnt.runDepProc: %v", err)
	}
	// Release waiters and allow them to wait on the underlying proc.
	tSpawnCond := sync.MakeCond(clnt.FsLib, path.Join(clnt.jobDir, COND+p.Pid), nil)
	tSpawnCond.Destroy()
}

func (clnt *DepProcClnt) getDepProc(pid string) (*DepProc, error) {
	b, _, err := clnt.GetFile(clnt.depProcFilePath(pid))
	if err != nil {
		return nil, err
	}

	p := MakeDepProc()
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Couldn't unmarshal waitfile: %v, %v", string(b), err)
		return nil, err
	}
	return p, nil
}

// Register start & exit dependencies in dependencies' waitfiles, and update the
// current proc's dependencies.
func (clnt *DepProcClnt) registerDependencies(p *DepProc) {
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

// Register a dependency on another the DepProc corresponding to pid. If the
// registration succeeded, return true. If the registration failed, assume the
// dependency has been satisfied, and return false.
func (clnt *DepProcClnt) registerDependant(pid string, dependant string, depType Tdep) bool {
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.depProcFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	p, err := clnt.getDepProc(pid)
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
		log.Fatalf("Unknown dep type in DepProcClnt.registerDependant: %v", depType)
	}

	// Write back updated deps
	b2, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling deps in ProcClnt.registerDependant: %v", err)
	}

	_, err = clnt.SetFile(clnt.depProcFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error setting waitfile in ProcClnt.registerDependant: %v, %v", clnt.depProcFilePath(pid), err)
	}

	return true
}

// Update dependants of the DepProc named by pid.
func (clnt *DepProcClnt) updateDependants(pid string, depType Tdep) {
	// Get the current contents of the wait file
	p, err := clnt.getDepProc(pid)
	if err != nil {
		db.DLPrintf("SCHEDCTL", "Error GetFile in DepProcClnt.updateDependants: %v, %v", clnt.depProcFilePath(pid), err)
		return
	}

	var dependants map[string]bool

	switch depType {
	case START_DEP:
		dependants = p.Dependants.StartDep
	case EXIT_DEP:
		dependants = p.Dependants.ExitDep
	default:
		log.Fatalf("Unknown depType in DepProcClnt.updateDependants: %v", depType)
	}

	for dependant, _ := range dependants {
		clnt.updateDependant(pid, dependant, depType)
	}

	// Record the start signal if applicable.
	if depType == START_DEP {
		p.Started = true
		b2, err := json.Marshal(p)
		if err != nil {
			log.Printf("Error marshalling depProcfile: %v", err)
			return
		}
		b2 = append(b2, ' ')
		_, err = clnt.SetFile(clnt.depProcFilePath(pid), b2, np.NoV)
		if err != nil {
			log.Printf("Error SetFile in DepProcClnt.updateDependants: %v, %v", clnt.depProcFilePath(pid), err)
		}
	}
}

// Update the dependency pid of dependant.
func (clnt *DepProcClnt) updateDependant(pid string, dependant string, depType Tdep) {
	// Create a lock to atomically update the job file.
	l := sync.MakeLock(clnt.FsLib, named.LOCKS, clnt.depProcFilePath(dependant), true)

	// Lock the job file to make sure we don't miss any dependency updates
	l.Lock()
	defer l.Unlock()

	p, err := clnt.getDepProc(dependant)
	if err != nil {
		log.Printf("Couldn't get waiter file in DepProcClnt.updateDependant: %v, %v", dependant, err)
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
		log.Fatalf("Unknown depType in DepProcClnt.updateDependant: %v", depType)
	}

	b2, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling in DepProcClnt.updateDependant: %v", err)
	}
	// XXX Hack around lack of OTRUNC
	for i := 0; i < DEPFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = clnt.SetFile(clnt.depProcFilePath(dependant), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing in ProcClnt.updateDependant: %v, %v", clnt.depProcFilePath(dependant), err)
	}

	if clnt.depProcIsRunnable(p) {
		clnt.runDepProc(p)
	}
}

// XXX REMOVE
func (clnt *DepProcClnt) SpawnNoOp(pid string, extiDep []string) error {
	log.Fatalf("SpawnNoOp not implemented")
	return nil
}

func (clnt *DepProcClnt) SwapExitDependency(pids []string) error {
	log.Fatalf("SwapExitDependency not implemented")
	return nil
}
