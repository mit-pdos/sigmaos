// Package procclnt implements the proc API
package procclnt

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/proc/kproc"
	beschedclnt "sigmaos/sched/besched/clnt"
	lcschedclnt "sigmaos/sched/lcsched/clnt"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	chunkclnt "sigmaos/sched/msched/proc/chunk/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/coordination/semaphore"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
)

type ProcClnt struct {
	sync.RWMutex
	*fslib.FsLib
	pid            sp.Tpid
	isExited       sp.Tpid
	procDirCreated bool
	mschedclnt     *mschedclnt.MSchedClnt
	beschedclnt    *beschedclnt.BESchedClnt
	lcschedclnt    *lcschedclnt.LCSchedClnt
	cs             *ChildState
	bins           *chunkclnt.BinPaths
}

func newProcClnt(fsl *fslib.FsLib, pid sp.Tpid, procDirCreated bool, kernelID string) *ProcClnt {
	clnt := &ProcClnt{
		FsLib:          fsl,
		pid:            pid,
		procDirCreated: procDirCreated,
		mschedclnt:     mschedclnt.NewMSchedClnt(fsl, kernelID),
		beschedclnt:    beschedclnt.NewBESchedClnt(fsl),
		lcschedclnt:    lcschedclnt.NewLCSchedClnt(fsl),
		cs:             newChildState(),
		bins:           chunkclnt.NewBinPaths(),
	}
	return clnt
}

// ========== SPAWN ==========

// Create the named state the proc (and its parent) expects.
func (clnt *ProcClnt) NewProc(p *proc.Proc, how proc.Thow, kernelId string) error {
	return clnt.spawn(kernelId, how, p)
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
		return kproc.RunKernelProc(clnt.ProcEnv().GetInnerContainerIP(), p, nil)
	}
	return nil, nil
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	// Set the named mount point if this isn't a privileged proc. If we were to
	// do this for a privileged proc, it could cause issues as it may save the
	// knamed address.
	if !p.IsPrivileged() {
		ep, err := clnt.GetNamedEndpoint()
		if err != nil {
			return err
		}
		p.SetNamedEndpoint(ep)
	}
	return clnt.spawn(sp.LOCAL, proc.HMSCHED, p)
}

// Spawn a proc on kernelId.
func (clnt *ProcClnt) spawn(kernelId string, how proc.Thow, p *proc.Proc) error {
	// Sanity check.
	if p.GetMcpu() > 0 && p.GetType() != proc.T_LC {
		db.DPrintf(db.ERROR, "Spawn non-LC proc with Mcpu set %v", p)
		return fmt.Errorf("Spawn non-LC proc with Mcpu set %v", p)
	}

	p.SetHow(how)

	if kid, ok := clnt.bins.GetBinKernelID(p.GetProgram()); ok {
		p.PrependSigmaPath(chunk.ChunkdPath(kid))
	}

	p.InheritParentProcEnv(clnt.ProcEnv())

	db.DPrintf(db.PROCCLNT, "Spawn [%v]: %v", kernelId, p)
	defer db.DPrintf(db.PROCCLNT, "Spawn done [%v]: %v", kernelId, p)
	if clnt.hasExited() != "" {
		db.DPrintf(db.PROCCLNT_ERR, "Spawn error called after Exited")
		db.DPrintf(db.ERROR, "Spawn error called after Exited")
		return fmt.Errorf("Spawn error called after Exited")
	}

	p.SetSpawnTime(time.Now())

	// Optionally spawn the proc through msched.
	if how == proc.HMSCHED {
		clnt.cs.Spawned(p.GetPid())
		// Transparently spawn in a background thread.
		go func() {
			db.DPrintf(db.PROCCLNT, "pre spawnRetry %v %v", kernelId, p)
			pseqno, err := clnt.spawnRetry(kernelId, p)
			db.DPrintf(db.PROCCLNT, "enqueued on besched %v and spawned on msched %v err %v proc %v", pseqno.GetProcqID(), pseqno.GetMSchedID(), err, p)
			clnt.cs.Started(p.GetPid(), pseqno, err)
			if err != nil {
				clnt.cleanupError(p.GetPid(), p.GetParentDir(), fmt.Errorf("Spawn error %v", err))
			}
		}()
	} else {
		clnt.cs.Spawned(p.GetPid())
		pseqno := proc.NewProcSeqno(sp.NOT_SET, kernelId, 0, 0)
		clnt.cs.Started(p.GetPid(), pseqno, nil)
		// Make the privileged proc's procdir
		err := clnt.MakeProcDir(p.GetPid(), p.GetProcDir())
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Err SpawnKernelProc MakeProcDir: %v", err)
			db.DPrintf(db.ERROR, "Err spawn MakeProcDir: %v", err)
			return err
		}
		// Create a semaphore to indicate a proc has started if this is a kernel
		// proc. Otherwise, msched will create the semaphore.
		kprocDir := proc.KProcDir(p.GetPid())
		semStart := semaphore.NewSemaphore(clnt.FsLib, filepath.Join(kprocDir, proc.START_SEM))
		semStart.Init(0)
	}
	return nil
}

func (clnt *ProcClnt) forceRunViaMSched(kernelID string, p *proc.Proc) error {
	err := clnt.mschedclnt.ForceRun(kernelID, false, p)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "forceRunViaMSched: getMSchedClnt %v err %v\n", kernelID, err)
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			db.DPrintf(db.PROCCLNT_ERR, "Unregister %v", kernelID)
			clnt.mschedclnt.UnregisterSrv(kernelID)
		}
		return err
	}
	return nil
}

func (clnt *ProcClnt) enqueueViaBESched(p *proc.Proc) (string, *proc.ProcSeqno, error) {
	start := time.Now()
	defer func(start time.Time) {
		perf.LogSpawnLatency("enqueueViaBESched", p.GetPid(), p.GetSpawnTime(), start)
	}(start)
	return clnt.beschedclnt.Enqueue(p)
}

func (clnt *ProcClnt) enqueueViaLCSched(p *proc.Proc) (string, error) {
	start := time.Now()
	defer func(start time.Time) {
		perf.LogSpawnLatency("enqueueViaLCSched", p.GetPid(), p.GetSpawnTime(), start)
	}(start)
	return clnt.lcschedclnt.Enqueue(p)
}

func (clnt *ProcClnt) spawnRetry(kernelId string, p *proc.Proc) (*proc.ProcSeqno, error) {
	start := time.Now()
	var pseqno *proc.ProcSeqno
	for i := 0; i < sp.Conf.Path.MAX_RESOLVE_RETRY; i++ {
		var err error
		if p.IsPrivileged() {
			// Privileged procs are force-run on the msched specified by kernelID in
			// order to make sure they end up on the correct msched
			err = clnt.forceRunViaMSched(kernelId, p)
			pseqno = proc.NewProcSeqno(sp.NOT_SET, kernelId, 0, 0)
		} else {
			if p.GetType() == proc.T_BE {
				// BE Non-kernel procs are enqueued via the besched.
				var mschedID string
				mschedID, pseqno, err = clnt.enqueueViaBESched(p)
				if err == nil {
					db.DPrintf(db.PROCCLNT, "spawn: SetBinKernelId proc %v seqno %v", p.GetProgram(), pseqno)
					start := time.Now()
					clnt.bins.SetBinKernelID(p.GetProgram(), pseqno.GetMSchedID())
					perf.LogSpawnLatency("SetBinKernelID", p.GetPid(), p.GetSpawnTime(), start)
					p.SetKernelID(pseqno.GetMSchedID(), false)
				} else if serr.IsErrorUnavailable(err) {
					clnt.bins.DelBinKernelID(p.GetProgram(), mschedID)
				}
			} else {
				// LC Non-kernel procs are enqueued via the besched.
				var spawnedMSchedID string
				spawnedMSchedID, err = clnt.enqueueViaLCSched(p)
				pseqno = proc.NewProcSeqno(sp.NOT_SET, spawnedMSchedID, 0, 0)
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
			db.DPrintf(db.PROCCLNT_ERR, "spawnRetry failed err %v proc %v", err, p)
			return nil, err
		}
		perf.LogSpawnLatency("spawnRetry", p.GetPid(), p.GetSpawnTime(), start)
		return pseqno, nil
	}
	db.DPrintf(db.PROCCLNT_ERR, "spawnRetry failed, too many retries (%v): %v", sp.Conf.Path.MAX_RESOLVE_RETRY, p)
	return nil, serr.NewErr(serr.TErrUnreachable, kernelId)
}

// ========== WAIT ==========

func (clnt *ProcClnt) waitStart(pid sp.Tpid, how proc.Thow) error {
	start := time.Now()
	defer func() {
		perf.LogSpawnLatency("WaitStart", pid, perf.TIME_NOT_SET, start)
	}()

	pseqno, err := clnt.cs.GetProcSeqno(pid)
	if err != nil {
		return fmt.Errorf("Unknown kernel ID for %v, %v", pid, err)
	}
	db.DPrintf(db.PROCCLNT, "WaitStart %v got kid %v", pid, pseqno.GetMSchedID())
	_, err = clnt.wait(mschedclnt.START, pid, pseqno.GetMSchedID(), pseqno, proc.START_SEM, how)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Err WaitStart %v %v", pid, err)
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid sp.Tpid) error {
	return clnt.waitStart(pid, proc.HMSCHED)
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStartKernelProc(pid sp.Tpid, how proc.Thow) error {
	return clnt.waitStart(pid, how)
}

func (clnt *ProcClnt) waitExit(pid sp.Tpid, how proc.Thow) (*proc.Status, error) {
	// Must wait for child to start.
	if err := clnt.waitStart(pid, how); err != nil {
		db.DPrintf(db.PROCCLNT, "waitStart err %v", err)
		return nil, err
	}
	pseqno, err := clnt.cs.GetProcSeqno(pid)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Unknown kernel ID %v", err)
		return nil, err
	}
	// Wait for proc to exit
	st, err := clnt.wait(mschedclnt.EXIT, pid, pseqno.GetMSchedID(), pseqno, proc.EXIT_SEM, how)
	// Mark proc as exited in local state
	clnt.cs.Exited(pid, st)
	if err != nil {
		return nil, err
	}

	status, err := clnt.getExitStatus(pid, how)
	return status, err
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExit(pid sp.Tpid) (*proc.Status, error) {
	return clnt.waitExit(pid, proc.HMSCHED)
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExitKernelProc(pid sp.Tpid, how proc.Thow) (*proc.Status, error) {
	return clnt.waitExit(pid, how)
}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid sp.Tpid) error {
	_, err := clnt.wait(mschedclnt.EVICT, pid, clnt.ProcEnv().GetKernelID(), proc.NewProcSeqno(sp.NOT_SET, clnt.ProcEnv().GetKernelID(), 0, 0), proc.EVICT_SEM, clnt.ProcEnv().GetHow())
	return err
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started() error {
	perf.LogSpawnLatency("procclnt.Started called", clnt.ProcEnv().GetPID(), clnt.ProcEnv().GetSpawnTime(), perf.TIME_NOT_SET)
	return clnt.notify(mschedclnt.START, clnt.ProcEnv().GetPID(), clnt.ProcEnv().GetKernelID(), proc.START_SEM, clnt.ProcEnv().GetHow(), nil, false)
}

// ========== EXITED ==========

// Proc pid mark itself as exited. Typically exited() is called by
// proc pid, but if the proc crashes, msched calls exited() on behalf
// of the failed proc. The exited proc abandons any chidren it may
// have. The exited proc cleans itself up.
//
// exited() should be called *once* per proc, but msched's procclnt may
// call exited() for other (crashed) procs.
func (clnt *ProcClnt) exited(procdir, parentdir, kernelID string, pid sp.Tpid, status *proc.Status, how proc.Thow, crashed bool) error {
	// Write the exit status
	if err := clnt.writeExitStatus(pid, parentdir, status, how); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "writeExitStatus err %v", err)
	}
	// Notify parent.
	err := clnt.notify(mschedclnt.EXIT, pid, kernelID, proc.EXIT_SEM, how, status, crashed)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error notify exited: %v", err)
	}
	// clean myself up
	r := removeProc(clnt.FsLib, procdir+"/", clnt.procDirCreated)
	if r != nil {
		return fmt.Errorf("Exited error [%v] %v", procdir, r)
	}
	return err
}

// Called voluntarily by the proc when it Exits normally.
func (clnt *ProcClnt) Exited(status *proc.Status) {
	db.DPrintf(db.PROCCLNT, "Exited normally %v parent %v pid %v status %v", clnt.ProcEnv().ProcDir, clnt.ProcEnv().ParentDir, clnt.ProcEnv().GetPID(), status)
	clnt.StopWatchingSrvs()
	// will catch some unintended misuses: a proc calling exited
	// twice or msched calling exited twice.
	if clnt.setExited(clnt.ProcEnv().GetPID()) == clnt.ProcEnv().GetPID() {
		b := debug.Stack()
		db.DFatalf("Exited called after exited %v stack:\n%v", clnt.ProcEnv().ProcDir, string(b))
	}
	err := clnt.exited(clnt.ProcEnv().ProcDir, clnt.ProcEnv().ParentDir, clnt.ProcEnv().GetKernelID(), clnt.ProcEnv().GetPID(), status, clnt.ProcEnv().GetHow(), false)
	if err != nil {
		db.DPrintf(db.ALWAYS, "exited %v err %v", clnt.ProcEnv().GetPID(), err)
		// failed to mark ourselves exited; crash to give procd/msched a chance
		// to cleanup
		crash.Crash()
	}
	db.DPrintf(db.PROCCLNT, "Done Exited normally")
}

// Stop the msched/besched/lcsched monitoring threads
func (clnt *ProcClnt) StopWatchingSrvs() {
	clnt.beschedclnt.StopWatching()
	clnt.lcschedclnt.StopWatching()
	clnt.mschedclnt.StopWatching()
}

// Called on behalf of the proc by msched when the proc crashes.
func (clnt *ProcClnt) ExitedCrashed(pid sp.Tpid, procdir string, parentdir string, status *proc.Status, how proc.Thow) {
	db.DPrintf(db.PROCCLNT, "Exited crashed %v parent %v pid %v status %v", procdir, parentdir, pid, status)
	err := clnt.exited(procdir, parentdir, "IGNORE_KERNEL_CRASHED", pid, status, how, true)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited %v err %v", pid, err)
	}
	// If proc ran, but crashed before calling Started, the parent may block indefinitely. Stop this from happening by calling semStart.Up()
	semPath := filepath.Join(parentdir, proc.START_SEM)
	if how != proc.HMSCHED {
		kprocDir := proc.KProcDir(pid)
		semPath = filepath.Join(kprocDir, proc.START_SEM)
	}
	semStart := semaphore.NewSemaphore(clnt.FsLib, semPath)
	semStart.Up()
}

// ========== EVICT ==========

func (clnt *ProcClnt) evictAt(pid sp.Tpid, mschedID string, how proc.Thow) error {
	return clnt.notify(mschedclnt.EVICT, pid, mschedID, proc.EVICT_SEM, how, nil, false)
}

func (clnt *ProcClnt) evict(pid sp.Tpid, how proc.Thow) error {
	pseqno, err := clnt.cs.GetProcSeqno(pid)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error Evict can't get kernel ID for proc: %v", err)
		return err
	}
	return clnt.evictAt(pid, pseqno.GetMSchedID(), how)
}

// Notifies a proc that it will be evicted using Evict. Called by parent.
func (clnt *ProcClnt) Evict(pid sp.Tpid) error {
	return clnt.evict(pid, proc.HMSCHED)
}

// For use by realmd when evicting procs for fairness
func (clnt *ProcClnt) EvictRealmProc(pid sp.Tpid, mschedID string) error {
	return clnt.evictAt(pid, mschedID, proc.HMSCHED)
}

func (clnt *ProcClnt) EvictKernelProc(pid sp.Tpid, how proc.Thow) error {
	return clnt.evict(pid, how)
}

func (clnt *ProcClnt) hasExited() sp.Tpid {
	clnt.RLock()
	defer clnt.RUnlock()
	return clnt.isExited
}

func (clnt *ProcClnt) setExited(pid sp.Tpid) sp.Tpid {
	clnt.Lock()
	defer clnt.Unlock()

	r := clnt.isExited
	clnt.isExited = pid
	return r
}
