package procclnt

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"runtime/debug"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kproc"
	"sigmaos/lcschedclnt"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/procqclnt"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ProcClnt struct {
	sync.Mutex
	*fslib.FsLib
	pid         sp.Tpid
	isExited    sp.Tpid
	scheddclnt  *scheddclnt.ScheddClnt
	procqclnt   *procqclnt.ProcQClnt
	lcschedclnt *lcschedclnt.LCSchedClnt
	cs          *ChildState
}

func newProcClnt(fsl *fslib.FsLib, pid sp.Tpid) *ProcClnt {
	clnt := &ProcClnt{
		FsLib:       fsl,
		pid:         pid,
		scheddclnt:  scheddclnt.NewScheddClnt(fsl),
		procqclnt:   procqclnt.NewProcQClnt(fsl),
		lcschedclnt: lcschedclnt.NewLCSchedClnt(fsl),
		cs:          newChildState(),
	}
	return clnt
}

// ========== SPAWN ==========

// Create the named state the proc (and its parent) expects.
func (clnt *ProcClnt) NewProc(p *proc.Proc, how proc.Thow, kernelId string) error {
	if how == proc.HSCHEDD {
		return clnt.spawn(kernelId, how, p, 0)
	} else {
		return clnt.spawn(kernelId, how, p, -1)
	}
}

func (clnt *ProcClnt) SpawnKernelProc(p *proc.Proc, how proc.Thow, kernelId string) (*exec.Cmd, error) {
	if err := clnt.NewProc(p, how, kernelId); err != nil {
		return nil, err
	}
	if how == proc.HLINUX {
		// If this proc wasn't intended to be spawned through procd, run it
		// as a local Linux process
		p.InheritParentProcEnv(clnt.ProcEnv())
		p.SetKernelID(kernelId, false)
		return kproc.RunKernelProc(clnt.ProcEnv().GetLocalIP(), p, nil)
	}
	return nil, nil
}

// Burst-spawn a set of procs across available procds. Return a slice of procs
// which were unable to be successfully spawned, as well as corresponding
// errors.
//
// Use of burstOffset news sure we continue rotating across invocations as
// well as within an invocation.
func (clnt *ProcClnt) SpawnBurst(ps []*proc.Proc, procsPerSchedd int) ([]*proc.Proc, []error) {
	failed := []*proc.Proc{}
	errs := []error{}
	for i := range ps {
		if err := clnt.spawn("", proc.HSCHEDD, ps[i], procsPerSchedd); err != nil {
			db.DPrintf(db.ALWAYS, "Error burst-spawn %v: %v", ps[i], err)
			failed = append(failed, ps[i])
			errs = append(errs, err)
		}
	}
	return failed, errs
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	return clnt.spawn("~local", proc.HSCHEDD, p, 0)
}

// Spawn a proc on kernelId. If spread > 0, p is part of SpawnBurt().
func (clnt *ProcClnt) spawn(kernelId string, how proc.Thow, p *proc.Proc, spread int) error {
	// Sanity check.
	if p.GetMcpu() > 0 && p.GetType() != proc.T_LC {
		db.DFatalf("Spawn non-LC proc with Mcpu set %v", p)
		return fmt.Errorf("Spawn non-LC proc with Mcpu set %v", p)
	}

	p.SetHow(how)

	p.InheritParentProcEnv(clnt.ProcEnv())

	db.DPrintf(db.PROCCLNT, "Spawn [%v]: %v", kernelId, p)
	if clnt.hasExited() != "" {
		db.DPrintf(db.PROCCLNT_ERR, "Spawn error called after Exited")
		db.DFatalf("Spawn error called after Exited")
	}

	if spread > 0 {
		// Update the list of active procds.
		clnt.scheddclnt.UpdateSchedds()
		// XXX For now, spread is ignored
		kid, err := clnt.scheddclnt.NextSchedd()
		if err != nil {
			return err
		}
		kernelId = kid
		if how != proc.HSCHEDD {
			db.DFatalf("Try to spread non-schedd proc")
		}
	}

	if err := clnt.addChild(p, p.GetParentDir(), how); err != nil {
		return err
	}

	p.SetSpawnTime(time.Now())
	// Optionally spawn the proc through schedd.
	if how == proc.HSCHEDD {
		clnt.cs.Spawned(p.GetPid())
		// Transparently spawn in a background thread.
		go func() {
			spawnedKernelID, err := clnt.spawnRetry(kernelId, p)
			clnt.cs.Started(p.GetPid(), spawnedKernelID, err)
			if err != nil {
				clnt.cleanupError(p.GetPid(), p.GetParentDir(), fmt.Errorf("Spawn error %v", err))
			}
		}()
	} else {
		clnt.cs.Spawned(p.GetPid())
		clnt.cs.Started(p.GetPid(), kernelId, nil)
		// Make the proc's procdir
		err := clnt.NewProcDir(p.GetPid(), p.GetProcDir(), p.IsPrivileged(), how)
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Err SpawnKernelProc NewProcDir: %v", err)
		}
		// Create a semaphore to indicate a proc has started if this is a kernel
		// proc. Otherwise, schedd will create the semaphore.
		kprocDir := proc.KProcDir(p.GetPid())
		semStart := semclnt.NewSemClnt(clnt.FsLib, path.Join(kprocDir, proc.START_SEM))
		semStart.Init(0)
	}
	return nil
}

func (clnt *ProcClnt) forceRunViaSchedd(kernelID string, p *proc.Proc) error {
	err := clnt.scheddclnt.ForceRun(kernelID, p)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "forceRunViaSchedd: getScheddClnt %v err %v\n", kernelID, err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.PROCCLNT_ERR, "Unregister %v", kernelID)
			clnt.scheddclnt.UnregisterSrv(kernelID)
		}
		return err
	}
	return nil
}

func (clnt *ProcClnt) enqueueViaProcQ(p *proc.Proc) (string, error) {
	return clnt.procqclnt.Enqueue(p)
}

func (clnt *ProcClnt) enqueueViaLCSched(p *proc.Proc) (string, error) {
	return clnt.lcschedclnt.Enqueue(p)
}

func (clnt *ProcClnt) spawnRetry(kernelId string, p *proc.Proc) (string, error) {
	s := time.Now()
	spawnedKernelID := procqclnt.NOT_ENQ
	for i := 0; i < pathclnt.MAXRETRY; i++ {
		var err error
		if p.IsPrivileged() {
			// Privileged procs are force-run on the schedd specified by kernelID in
			// order to make sure they end up on the correct scheddd
			err = clnt.forceRunViaSchedd(kernelId, p)
			spawnedKernelID = kernelId
		} else {
			if p.GetType() == proc.T_BE {
				// BE Non-kernel procs are enqueued via the procq.
				spawnedKernelID, err = clnt.enqueueViaProcQ(p)
			} else {
				// LC Non-kernel procs are enqueued via the procq.
				spawnedKernelID, err = clnt.enqueueViaLCSched(p)
			}
		}
		// If spawn attempt resulted in an error, check if it was due to the
		// server becoming unreachable.
		if err != nil {
			// If unreachable, retry.
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.PROCCLNT_ERR, "Err spawnRetry unreachable %v", err)
				continue
			}
			return spawnedKernelID, err
		}
		db.DPrintf(db.SPAWN_LAT, "[%v] E2E Spawn RPC %v", p.GetPid(), time.Since(s))
		return spawnedKernelID, nil
	}
	return spawnedKernelID, serr.NewErr(serr.TErrUnreachable, kernelId)
}

// ========== WAIT ==========

func (clnt *ProcClnt) waitStart(pid sp.Tpid, how proc.Thow) error {
	s := time.Now()
	defer db.DPrintf(db.SPAWN_LAT, "[%v] E2E WaitStart %v", pid, time.Since(s))

	kernelID, err := clnt.cs.GetKernelID(pid)
	if err != nil {
		return fmt.Errorf("Unknown kernel ID %v", err)
	}
	err = clnt.wait(scheddclnt.START, pid, kernelID, proc.START_SEM, how)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Err WaitStart %v %v", pid, err)
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid sp.Tpid) error {
	return clnt.waitStart(pid, proc.HSCHEDD)
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStartKernelProc(pid sp.Tpid, how proc.Thow) error {
	return clnt.waitStart(pid, how)
}

func (clnt *ProcClnt) waitExit(pid sp.Tpid, how proc.Thow) (*proc.Status, error) {
	// Must wait for child to fill in return status pipe.
	if err := clnt.waitStart(pid, how); err != nil {
		db.DPrintf(db.PROCCLNT, "waitStart err %v", err)
		return nil, err
	}
	kernelID, err := clnt.cs.GetKernelID(pid)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Unknown kernel ID %v", err)
		return nil, err
	}
	defer clnt.cs.Exited(pid)
	err = clnt.wait(scheddclnt.EXIT, pid, kernelID, proc.EXIT_SEM, how)

	defer clnt.RemoveChild(pid)

	childDir := path.Dir(proc.GetChildProcDir(proc.PROCDIR, pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.EXIT_STATUS))
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Missing return status, schedd must have crashed: %v, %v", pid, err)
		return nil, fmt.Errorf("Missing return status, schedd must have crashed: %v", err)
	}

	status := &proc.Status{}
	if err := json.Unmarshal(b, status); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "waitexit unmarshal err %v", err)
		return nil, err
	}

	return status, nil
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExit(pid sp.Tpid) (*proc.Status, error) {
	return clnt.waitExit(pid, proc.HSCHEDD)
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExitKernelProc(pid sp.Tpid, how proc.Thow) (*proc.Status, error) {
	return clnt.waitExit(pid, how)
}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid sp.Tpid) error {
	return clnt.wait(scheddclnt.EVICT, pid, clnt.ProcEnv().GetKernelID(), proc.EVICT_SEM, clnt.ProcEnv().GetHow())
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started() error {
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc started %v", clnt.ProcEnv().GetPID(), time.Now())
	return clnt.notify(scheddclnt.START, clnt.ProcEnv().GetPID(), clnt.ProcEnv().GetKernelID(), proc.START_SEM, clnt.ProcEnv().GetHow(), false)
}

// ========== EXITED ==========

// Proc pid mark itself as exited. Typically exited() is called by
// proc pid, but if the proc crashes, schedd calls exited() on behalf
// of the failed proc. The exited proc abandons any chidren it may
// have. The exited proc cleans itself up.
//
// exited() should be called *once* per proc, but schedd's procclnt may
// call exited() for other (crashed) procs.

func (clnt *ProcClnt) exited(procdir, parentdir, kernelID string, pid sp.Tpid, status *proc.Status, how proc.Thow, crashed bool) error {
	b, err := json.Marshal(status)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited marshal err %v", err)
		return err
	}
	// May return an error if parent already exited.
	fn := path.Join(parentdir, proc.EXIT_STATUS)
	if _, err := clnt.PutFile(fn, 0777, sp.OWRITE, b); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited error (parent already exited) NewFile %v err %v", fn, err)
	}
	// Notify parent.
	err = clnt.notify(scheddclnt.EXIT, pid, kernelID, proc.EXIT_SEM, how, crashed)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error notify exited: %v", err)
	}
	// clean myself up
	r := removeProc(clnt.FsLib, procdir+"/")
	if r != nil {
		return fmt.Errorf("Exited error [%v] %v", procdir, r)
	}
	return nil
}

// Called voluntarily by the proc when it Exits normally.
func (clnt *ProcClnt) Exited(status *proc.Status) {
	db.DPrintf(db.PROCCLNT, "Exited normally %v parent %v pid %v status %v", clnt.ProcEnv().ProcDir, clnt.ProcEnv().ParentDir, clnt.ProcEnv().GetPID(), status)
	// will catch some unintended misuses: a proc calling exited
	// twice or schedd calling exited twice.
	if clnt.setExited(clnt.ProcEnv().GetPID()) == clnt.ProcEnv().GetPID() {
		b := debug.Stack()
		db.DFatalf("Exited called after exited %v stack:\n%v", clnt.ProcEnv().ProcDir, string(b))
	}
	err := clnt.exited(clnt.ProcEnv().ProcDir, clnt.ProcEnv().ParentDir, clnt.ProcEnv().GetKernelID(), clnt.ProcEnv().GetPID(), status, clnt.ProcEnv().GetHow(), false)
	if err != nil {
		db.DPrintf(db.ALWAYS, "exited %v err %v", clnt.ProcEnv().GetPID(), err)
	}
}

// Called on behalf of the proc by schedd when the proc crashes.
func (clnt *ProcClnt) ExitedCrashed(pid sp.Tpid, procdir string, parentdir string, status *proc.Status, how proc.Thow) {
	db.DPrintf(db.PROCCLNT, "Exited crashed %v parent %v pid %v status %v", procdir, parentdir, pid, status)
	err := clnt.exited(procdir, parentdir, "IGNORE_KERNEL_CRASHED", pid, status, how, true)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited %v err %v", pid, err)
	}
	// If proc ran, but crashed before calling Started, the parent may block indefinitely. Stop this from happening by calling semStart.Up()
	semPath := path.Join(parentdir, proc.START_SEM)
	if how != proc.HSCHEDD {
		kprocDir := proc.KProcDir(pid)
		semPath = path.Join(kprocDir, proc.START_SEM)
	}
	semStart := semclnt.NewSemClnt(clnt.FsLib, semPath)
	semStart.Up()
}

// ========== EVICT ==========

func (clnt *ProcClnt) evict(pid sp.Tpid, how proc.Thow) error {
	kernelID, err := clnt.cs.GetKernelID(pid)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error Evict can't get kernel ID for proc: %v", err)
		return err
	}
	return clnt.notify(scheddclnt.EVICT, pid, kernelID, proc.EVICT_SEM, how, false)
}

// Notifies a proc that it will be evicted using Evict. Called by parent.
func (clnt *ProcClnt) Evict(pid sp.Tpid) error {
	return clnt.evict(pid, proc.HSCHEDD)
}

func (clnt *ProcClnt) EvictKernelProc(pid sp.Tpid, how proc.Thow) error {
	return clnt.evict(pid, how)
}

func (clnt *ProcClnt) hasExited() sp.Tpid {
	clnt.Lock()
	defer clnt.Unlock()
	return clnt.isExited
}

func (clnt *ProcClnt) setExited(pid sp.Tpid) sp.Tpid {
	clnt.Lock()
	defer clnt.Unlock()
	r := clnt.isExited
	clnt.isExited = pid
	return r
}
