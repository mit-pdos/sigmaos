package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/fslib"
	np "ulambda/ninep"
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
	// XXX REMOVE BY IMPLEMENTING TRUNC
	WAITFILE_PADDING = 1000
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

//type Proc struct {
//	Pid        string          // SigmaOS PID
//	Program    string          // Program to run
//	WDir       string          // Working directory for the process
//	Args       []string        // Args
//	Env        []string        // Environment variables
//	StartDep   []string        // Start dependencies // XXX Replace somehow?
//	ExitDep    map[string]bool // Exit dependencies// XXX Replace somehow?
//	StartTimer uint32          // Start timer in seconds
//	Type       Ttype           // Type
//	Ncore      Tcore           // Number of cores requested
//}

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

// ========== SPAWN ==========

func (pctl *ProcCtl) Spawn(p *fslib.Attr) error {
	// Create a file for waiters to watch & wait on
	err := pctl.makeWaitFile(p.Pid)
	if err != nil {
		return err
	}
	pctl.pruneExitDeps(p)
	b, err := json.Marshal(p)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		pctl.removeWaitFile(p.Pid)
		return err
	}
	err = pctl.MakeFileAtomic(path.Join(WAITQ, p.Pid), 0777, b)
	if err != nil {
		return err
	}
	// Notify localds that a job has become runnable
	pctl.SignalNewJob()
	return nil
}

// Notify localds that a job has become runnable
func (pctl *ProcCtl) SignalNewJob() error {
	// Needs to be done twice, since someone waiting on the signal will create a
	// new lock file, even if they've crashed
	return pctl.UnlockFile(fslib.LOCKS, fslib.JOB_SIGNAL)
}

func (pctl *ProcCtl) makeWaitFile(pid string) error {
	fpath := waitFilePath(pid)
	var wf fslib.WaitFile
	wf.Started = false
	b, err := json.Marshal(wf)
	if err != nil {
		log.Printf("Error marshalling waitfile: %v", err)
	}
	// XXX hack around lack of OTRUNC
	for i := 0; i < WAITFILE_PADDING; i++ {
		b = append(b, ' ')
	}
	// Make a writable, versioned file
	err = pctl.MakeFile(fpath, 0777, np.OWRITE, b)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() != "EOF" {
		return fmt.Errorf("Error on MakeFile MakeWaitFile %v: %v", fpath, err)
	}
	return nil
}

// XXX When we start handling large numbers of lambdas, may be better to stat
// each exit dep individually. For now, this is more efficient (# of RPCs).
// If we know nothing about an exit dep, ignore it by marking it as exited
func (pctl *ProcCtl) pruneExitDeps(p *fslib.Attr) {
	spawned := pctl.getSpawnedLambdas()
	for pid, _ := range p.ExitDep {
		if _, ok := spawned[waitFileName(pid)]; !ok {
			p.ExitDep[pid] = true
		}
	}
}

func (pctl *ProcCtl) removeWaitFile(pid string) error {
	fpath := waitFilePath(pid)
	err := pctl.Remove(fpath)
	if err != nil {
		log.Printf("Error on RemoveWaitFile  %v: %v", fpath, err)
		return err
	}
	return nil
}

func waitFilePath(pid string) string {
	return path.Join(SPAWNED, waitFileName(pid))
}

func waitFileName(pid string) string {
	return fslib.LockName(WAIT_LOCK + pid)
}

func (pctl *ProcCtl) getSpawnedLambdas() map[string]bool {
	d, err := pctl.ReadDir(SPAWNED)
	if err != nil {
		log.Printf("Error reading spawned dir in pruneExitDeps: %v", err)
	}
	spawned := map[string]bool{}
	for _, l := range d {
		spawned[l.Name] = true
	}
	return spawned
}

// ========== STARTED ==========

/*
 * PairDep-based lambdas are runnable only if they are the producer (whoever
 * claims and runs the producer will also start the consumer, so we disallow
 * unilaterally claiming the consumer for now), and only once all of their
 * consumers have been started. For now we assume that
 * consumers only have one producer, and the roles of producer and consumer
 * are mutually exclusive. We also expect (though not strictly necessary)
 * that producers only have one consumer each. If this is no longer the case,
 * we should handle oversubscription more carefully.
 */
func (pctl *ProcCtl) Started(pid string) error {
	pctl.setWaitFileStarted(pid, true)
	return nil
}

func (pctl *ProcCtl) setWaitFileStarted(pid string, started bool) {
	pctl.LockFile(fslib.LOCKS, waitFilePath(pid))
	defer pctl.UnlockFile(fslib.LOCKS, waitFilePath(pid))

	// Get the current contents of the file & its version
	b1, _, err := pctl.GetFile(waitFilePath(pid))
	if err != nil {
		log.Printf("Error reading when registerring retstat: %v, %v", waitFilePath(pid), err)
		return
	}
	var wf fslib.WaitFile
	err = json.Unmarshal(b1, &wf)
	if err != nil {
		log.Fatalf("Error unmarshalling waitfile: %v, %v", string(b1), err)
		return
	}
	wf.Started = started
	b2, err := json.Marshal(wf)
	if err != nil {
		log.Printf("Error marshalling waitfile: %v", err)
		return
	}
	// XXX hack around lack of OTRUNC
	for i := 0; i < WAITFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = pctl.SetFile(waitFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing when registerring retstat: %v, %v", waitFilePath(pid), err)
	}
}

// ========== EXITING ==========

func (pctl *ProcCtl) Exiting(pid string, status string) error {
	pctl.wakeupExit(pid)
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

func (pctl *ProcCtl) wakeupExit(pid string) error {
	err := pctl.modifyExitDependencies(func(deps map[string]bool) bool {
		if _, ok := deps[pid]; ok {
			deps[pid] = true
			return true
		}
		return false
	})
	if err != nil {
		return err
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
	err := pctl.SetRemoveWatch(waitFilePath(pid), func(p string, err error) {
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

// XXX REMOVE
// ========== SPAWN_NO_OP =========

// Spawn a no-op lambda
func (pctl *ProcCtl) SpawnNoOp(pid string, exitDep []string) error {
	a := &fslib.Attr{}
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
		var attr fslib.Attr
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
