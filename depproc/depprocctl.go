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

type DepProcCtl struct {
	JobID  string
	jobDir string
	proc.ProcCtl
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

func MakeDepProcCtl(fsl *fslib.FsLib, ctl proc.ProcCtl) *DepProcCtl {
	jid := DEFAULT_JOB_ID
	dctl := &DepProcCtl{}
	dctl.JobID = jid
	dctl.ProcCtl = ctl
	dctl.FsLib = fsl
	dctl.jobDir = path.Join(JOBS, jid)

	MakeJob(fsl, DEFAULT_JOB_ID)
	usingDepProc = true

	return dctl
}

// ========== NAMING CONVENTIONS ==========

func (ctl *DepProcCtl) depProcFilePath(pid string) string {
	return path.Join(ctl.jobDir, pid)
}

func (ctl *DepProcCtl) depFilePath(pid string) string {
	return path.Join(ctl.jobDir, DEPFILE+pid)
}

// ========== SPAWN ==========

func (ctl *DepProcCtl) Spawn(gp proc.GenericProc) error {
	var p *DepProc
	switch gp.(type) {
	case *DepProc:
		p = gp.(*DepProc)
	case *proc.Proc:
		p = MakeDepProc()
		p.Proc = gp.(*proc.Proc)
	}
	depProcFPath := path.Join(ctl.jobDir, p.Pid)

	// If the underlying proc hasn't been spawned yet, the Waits will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(ctl.FsLib, path.Join(ctl.jobDir, COND+p.Pid), nil)
	tSpawnCond.Init()

	// Create a lock to make sure we don't miss updates from depProcs we depend on.
	tLock := sync.MakeLock(ctl.FsLib, named.LOCKS, depProcFPath, true)

	// Lock the depProc file to make sure we don't miss any dependency updates.
	tLock.Lock()
	defer tLock.Unlock()

	// Register dependency backwards pointers.
	ctl.registerDependencies(p)

	// Atomically create the depProc file.
	err := atomic.MakeFileJsonAtomic(ctl.FsLib, depProcFPath, 0777, p)
	if err != nil {
		// Release waiters if spawn fails.
		tSpawnCond.Destroy()
		return err
	}

	// Start the depProc if it is runnable
	if ctl.depProcIsRunnable(p) {
		ctl.runDepProc(p)
	}

	return nil
}

// ========== WAIT ==========

// Wait for a depProc to start
func (ctl *DepProcCtl) WaitStart(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitStart will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(ctl.FsLib, path.Join(ctl.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return ctl.ProcCtl.WaitStart(pid)
}

// Wait for a depProc to exit
func (ctl *DepProcCtl) WaitExit(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitExit will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(ctl.FsLib, path.Join(ctl.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return ctl.ProcCtl.WaitExit(pid)
}

// Wait for a depProc to evict
func (ctl *DepProcCtl) WaitEvict(pid string) error {
	// If the underlying proc hasn't been spawned yet, the WaitEvict will fall
	// through. This condition variable fires (and is destroyed) once the
	// underlying proc is spawned, so we don't accidentally fall through early.
	tSpawnCond := sync.MakeCond(ctl.FsLib, path.Join(ctl.jobDir, COND+pid), nil)
	tSpawnCond.Wait()
	return ctl.ProcCtl.WaitEvict(pid)
}

// ========== STARTED ==========

func (ctl *DepProcCtl) Started(pid string) error {
	// Lock the depProc file
	l := sync.MakeLock(ctl.FsLib, named.LOCKS, ctl.depProcFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	// Update depProcs that depend on this depProc.
	ctl.updateDependants(pid, START_DEP)
	ctl.ProcCtl.Started(pid)

	return nil
}

// ========== EXITED ==========

func (ctl *DepProcCtl) Exited(pid string) error {
	// Lock the depProc file
	l := sync.MakeLock(ctl.FsLib, named.LOCKS, ctl.depProcFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	// Update depProcs that depend on this depProc.
	ctl.updateDependants(pid, EXIT_DEP)
	ctl.ProcCtl.Exited(pid)

	err := ctl.Remove(ctl.depProcFilePath(pid))
	if err != nil {
		if usingDepProc {
			db.DLPrintf("DEPPROC", "Error removing depProc file in DepProcCtl.Exited: %v", err)
		} else {
			log.Printf("Error removing depProc file in DepProcCtl.Exited: %v", err)
		}
		return err
	}

	return nil
}

// ========== EVICTED ==========

func (ctl *DepProcCtl) Evict(pid string) error {
	return ctl.ProcCtl.Evict(pid)
}

// ========== HELPERS ==========

func (ctl *DepProcCtl) depProcIsRunnable(p *DepProc) bool {
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

func (ctl *DepProcCtl) runDepProc(p *DepProc) {
	err := ctl.ProcCtl.Spawn(p.Proc)
	if err != nil {
		log.Fatalf("Error spawning depProc in DepProcCtl.runDepProc: %v", err)
	}
	// Release waiters and allow them to wait on the underlying proc.
	tSpawnCond := sync.MakeCond(ctl.FsLib, path.Join(ctl.jobDir, COND+p.Pid), nil)
	tSpawnCond.Destroy()
}

func (ctl *DepProcCtl) getDepProc(pid string) (*DepProc, error) {
	b, _, err := ctl.GetFile(ctl.depProcFilePath(pid))
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
func (ctl *DepProcCtl) registerDependencies(p *DepProc) {
	for dep, _ := range p.Dependencies.StartDep {
		if ok := ctl.registerDependant(dep, p.Pid, START_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			p.Dependencies.StartDep[dep] = true
		}
	}
	for dep, _ := range p.Dependencies.ExitDep {
		if ok := ctl.registerDependant(dep, p.Pid, EXIT_DEP); !ok {
			// If we failed to register the dependency, assume the dependency has
			// already been satisfied.
			p.Dependencies.ExitDep[dep] = true
		}
	}
}

// Register a dependency on another the DepProc corresponding to pid. If the
// registration succeeded, return true. If the registration failed, assume the
// dependency has been satisfied, and return false.
func (ctl *DepProcCtl) registerDependant(pid string, dependant string, depType Tdep) bool {
	l := sync.MakeLock(ctl.FsLib, named.LOCKS, ctl.depProcFilePath(pid), true)

	l.Lock()
	defer l.Unlock()

	p, err := ctl.getDepProc(pid)
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
		log.Fatalf("Unknown dep type in DepProcCtl.registerDependant: %v", depType)
	}

	// Write back updated deps
	b2, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling deps in ProcCtl.registerDependant: %v", err)
	}

	_, err = ctl.SetFile(ctl.depProcFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error setting waitfile in ProcCtl.registerDependant: %v, %v", ctl.depProcFilePath(pid), err)
	}

	return true
}

// Update dependants of the DepProc named by pid.
func (ctl *DepProcCtl) updateDependants(pid string, depType Tdep) {
	// Get the current contents of the wait file
	p, err := ctl.getDepProc(pid)
	if err != nil {
		db.DLPrintf("SCHEDCTL", "Error GetFile in DepProcCtl.updateDependants: %v, %v", ctl.depProcFilePath(pid), err)
		return
	}

	var dependants map[string]bool

	switch depType {
	case START_DEP:
		dependants = p.Dependants.StartDep
	case EXIT_DEP:
		dependants = p.Dependants.ExitDep
	default:
		log.Fatalf("Unknown depType in DepProcCtl.updateDependants: %v", depType)
	}

	for dependant, _ := range dependants {
		ctl.updateDependant(pid, dependant, depType)
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
		_, err = ctl.SetFile(ctl.depProcFilePath(pid), b2, np.NoV)
		if err != nil {
			log.Printf("Error SetFile in DepProcCtl.updateDependants: %v, %v", ctl.depProcFilePath(pid), err)
		}
	}
}

// Update the dependency pid of dependant.
func (ctl *DepProcCtl) updateDependant(pid string, dependant string, depType Tdep) {
	// Create a lock to atomically update the job file.
	l := sync.MakeLock(ctl.FsLib, named.LOCKS, ctl.depProcFilePath(dependant), true)

	// Lock the job file to make sure we don't miss any dependency updates
	l.Lock()
	defer l.Unlock()

	p, err := ctl.getDepProc(dependant)
	if err != nil {
		log.Printf("Couldn't get waiter file in DepProcCtl.updateDependant: %v, %v", dependant, err)
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
		log.Fatalf("Unknown depType in DepProcCtl.updateDependant: %v", depType)
	}

	b2, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling in DepProcCtl.updateDependant: %v", err)
	}
	// XXX Hack around lack of OTRUNC
	for i := 0; i < DEPFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = ctl.SetFile(ctl.depProcFilePath(dependant), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing in ProcCtl.updateDependant: %v, %v", ctl.depProcFilePath(dependant), err)
	}

	if ctl.depProcIsRunnable(p) {
		ctl.runDepProc(p)
	}
}

// XXX REMOVE
func (ctl *DepProcCtl) SpawnNoOp(pid string, extiDep []string) error {
	log.Fatalf("SpawnNoOp not implemented")
	return nil
}

func (ctl *DepProcCtl) SwapExitDependency(pids []string) error {
	log.Fatalf("SwapExitDependency not implemented")
	return nil
}
