package procclnt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"runtime/debug"
	"sync"
	"time"

	//	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kproc"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	schedd "sigmaos/schedd/proto"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Thow uint32

const (
	HSCHEDD Thow = iota + 1 // spawned as a sigmos proc
	HLINUX                  // spawned as a linux process
	HDOCKER                 // spawned as a container
)

type ProcClnt struct {
	sync.Mutex
	*fslib.FsLib
	pid        sp.Tpid
	isExited   sp.Tpid
	procdir    string
	scheddclnt *scheddclnt.ScheddClnt
}

func makeProcClnt(fsl *fslib.FsLib, pid sp.Tpid, procdir string) *ProcClnt {
	clnt := &ProcClnt{}
	clnt.FsLib = fsl
	clnt.pid = pid
	clnt.procdir = procdir
	clnt.scheddclnt = scheddclnt.MakeScheddClnt(fsl)
	return clnt
}

// ========== SPAWN ==========

// Create the named state the proc (and its parent) expects.
func (clnt *ProcClnt) MkProc(p *proc.Proc, how Thow, kernelId string) error {
	if how == HSCHEDD {
		return clnt.spawn(kernelId, how, p, 0)
	} else {
		return clnt.spawn(kernelId, how, p, -1)
	}
}

func (clnt *ProcClnt) SpawnKernelProc(p *proc.Proc, how Thow, kernelId string) (*exec.Cmd, error) {
	if err := clnt.MkProc(p, how, kernelId); err != nil {
		return nil, err
	}
	if how == HLINUX {
		// If this proc wasn't intended to be spawned through procd, run it
		// as a local Linux process
		return kproc.RunKernelProc(clnt.SigmaConfig(), p, clnt.Realm(), nil)
	}
	return nil, nil
}

// Burst-spawn a set of procs across available procds. Return a slice of procs
// which were unable to be successfully spawned, as well as corresponding
// errors.
//
// Use of burstOffset makes sure we continue rotating across invocations as
// well as within an invocation.
func (clnt *ProcClnt) SpawnBurst(ps []*proc.Proc, procsPerSchedd int) ([]*proc.Proc, []error) {
	failed := []*proc.Proc{}
	errs := []error{}
	for i := range ps {
		if err := clnt.spawn("", HSCHEDD, ps[i], procsPerSchedd); err != nil {
			db.DPrintf(db.ALWAYS, "Error burst-spawn %v: %v", ps[i], err)
			failed = append(failed, ps[i])
			errs = append(errs, err)
		}
	}
	return failed, errs
}

type errTuple struct {
	proc *proc.Proc
	err  error
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	return clnt.spawn("~local", HSCHEDD, p, 0)
}

// Spawn a proc on kernelId. If spread > 0, p is part of SpawnBurt().
func (clnt *ProcClnt) spawn(kernelId string, how Thow, p *proc.Proc, spread int) error {
	if p.GetMcpu() > 0 && p.GetType() != proc.T_LC {
		db.DFatalf("Spawn non-LC proc with Mcpu set %v", p)
		return fmt.Errorf("Spawn non-LC proc with Mcpu set %v", p)
	}

	// XXX set other fields? procdir etc?
	childCfg := proc.NewChildSigmaConfig(clnt.SigmaConfig(), p)
	p.SetSigmaConfig(childCfg)
	p.Finalize(kernelId)

	// Set the realm id.
	if p.RealmStr == "" {
		p.SetRealm(clnt.Realm())
	}

	// Set the parent dir
	p.SetParentDir(clnt.procdir)
	childProcdir := p.ProcDir

	db.DPrintf(db.PROCCLNT, "Spawn [%v]: %v", kernelId, p)
	if clnt.hasExited() != "" {
		db.DPrintf(db.PROCCLNT_ERR, "Spawn error called after Exited")
		db.DFatalf("Spawn error called after Exited")
	}

	// XXX need to pick kernelId now
	if spread > 0 {
		// Update the list of active procds.
		clnt.scheddclnt.UpdateSchedds()
		kernelId = clnt.scheddclnt.NextSchedd(spread)
	}

	if err := clnt.addChild(kernelId, p, childProcdir, how); err != nil {
		return err
	}

	p.SetSpawnTime(time.Now())
	// If this is not a privileged proc, spawn it through schedd.
	if how == HSCHEDD {
		if err := clnt.spawnRetry(kernelId, p); err != nil {
			return clnt.cleanupError(p.GetPid(), childProcdir, fmt.Errorf("Spawn error %v", err))
		}
	} else {
		// Make the proc's procdir
		err := clnt.MakeProcDir(p.GetPid(), p.ProcDir, p.IsPrivilegedProc())
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Err SpawnKernelProc MakeProcDir: %v", err)
		}
		// Create a semaphore to indicate a proc has started if this is a kernel
		// proc. Otherwise, schedd will create the semaphore.
		childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, p.GetPid()))
		semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
		semStart.Init(0)
	}
	return nil
}

func (clnt *ProcClnt) spawnRetry(kernelId string, p *proc.Proc) error {
	s := time.Now()
	for i := 0; i < pathclnt.MAXRETRY; i++ {
		rpcc, err := clnt.getScheddClnt(kernelId)
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "spawnRetry: getScheddClnt %v err %v\n", kernelId, err)
			return err
		}
		req := &schedd.SpawnRequest{
			Realm:     clnt.Realm().String(),
			ProcProto: p.GetProto(),
		}
		res := &schedd.SpawnResponse{}
		if err := rpcc.RPC("Schedd.Spawn", req, res); err != nil {
			db.DPrintf(db.ALWAYS, "Schedd.Spawn %v err %v\n", kernelId, err)
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ALWAYS, "Force lookup %v\n", kernelId)
				clnt.scheddclnt.UnregisterClnt(kernelId)
				continue
			}
			return err
		}
		db.DPrintf(db.SPAWN_LAT, "[%v] E2E Spawn RPC %v", p.GetPid(), time.Since(s))
		return nil
	}
	return serr.MkErr(serr.TErrUnreachable, kernelId)
}

func (clnt *ProcClnt) getScheddClnt(kernelId string) (*rpcclnt.RPCClnt, error) {
	rpcc, err := clnt.scheddclnt.GetScheddClnt(kernelId)
	if err != nil {
		return nil, err
	}
	// Local schedd is special: it has two entries, one under its
	// kernelId and the other one under ~local.
	if kernelId == "~local" {
		if err := clnt.scheddclnt.RegisterLocalClnt(rpcc); err != nil {
			db.DFatalf("RegisterLocalClnt err %v\n", err)
			return rpcc, err
		}
	}
	return rpcc, nil
}

// ========== WAIT ==========

// Wait until a proc file is removed. Return an error if SetRemoveWatch returns
// an unreachable error.
func (clnt *ProcClnt) waitProcFileRemove(pid sp.Tpid, pn string) error {
	db.DPrintf(db.PROCCLNT, "%v set remove watch: %v", pid, pn)
	done := make(chan bool)
	err := clnt.SetRemoveWatch(pn, func(string, error) {
		done <- true
	})
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error waitStart SetRemoveWatch %v", err)
		var sr *serr.Err
		if errors.As(err, &sr) && (sr.IsErrUnreachable() || sr.IsMaybeSpecialElem()) {
			return err
		}
	} else {
		<-done
	}
	return nil
}

func (clnt *ProcClnt) waitStart(pid sp.Tpid) error {
	childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.PROCFILE_LINK))
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Can't get procip file: %v", err)
		return err
	}
	procfileLink := string(b)
	// Kernel procs will have empty proc file links.
	if procfileLink != "" {
		// Wait for the proc queue file to be removed. Should not return an error.
		if err := clnt.waitProcFileRemove(pid, procfileLink); err != nil {
			return err
		}
	}
	db.DPrintf(db.PROCCLNT, "WaitStart %v %v", pid, childDir)
	defer db.DPrintf(db.PROCCLNT, "WaitStart done waiting %v %v", pid, childDir)
	s := time.Now()
	defer db.DPrintf(db.SPAWN_LAT, "[%v] E2E Semaphore Down %v", pid, time.Since(s))
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
	return semStart.Down()
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid sp.Tpid) error {
	err := clnt.waitStart(pid)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "WaitStart %v %v", pid, err)
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExit(pid sp.Tpid) (*proc.Status, error) {
	// Must wait for child to fill in return status pipe.
	if err := clnt.waitStart(pid); err != nil {
		db.DPrintf(db.PROCCLNT, "waitStart err %v", err)
	}

	db.DPrintf(db.PROCCLNT, "WaitExit %v", pid)

	// Make sure the child proc has exited.
	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(proc.GetChildProcDir(clnt.procdir, pid), proc.EXIT_SEM))
	if err := semExit.Down(); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error WaitExit semExit.Down: %v", err)
		return nil, fmt.Errorf("Error semExit.Down: %v", err)
	}

	defer clnt.RemoveChild(pid)

	childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, pid))
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

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid sp.Tpid) error {
	db.DPrintf(db.PROCCLNT, "WaitEvict %v", pid)
	procdir := proc.PROCDIR
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Down()
	if err != nil {
		return fmt.Errorf("WaitEvict error %v", err)
	}
	db.DPrintf(db.PROCCLNT, "WaitEvict evicted %v", pid)
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started() error {
	db.DPrintf(db.PROCCLNT, "Started %v", clnt.pid)
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc started %v", clnt.SigmaConfig().PID, time.Now())

	// Link self into parent dir
	if err := clnt.linkSelfIntoParentDir(); err != nil {
		db.DPrintf(db.PROCCLNT, "linkSelfIntoParentDir %v err %v", clnt.pid, err)
		return err
	}

	// Mark self as started
	semPath := path.Join(proc.PARENTDIR, proc.START_SEM)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, semPath)
	err := semStart.Up()
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Started error %v %v", semPath, err)
	}
	// File may not be found if parent exited first or isn't reachable
	if err != nil && !serr.IsErrorUnavailable(err) {
		return fmt.Errorf("Started error %v", err)
	}
	return nil
}

// ========== EXITED ==========

// Proc pid mark itself as exited. Typically exited() is called by
// proc pid, but if the proc crashes, schedd calls exited() on behalf
// of the failed proc. The exited proc abandons any chidren it may
// have. The exited proc cleans itself up.
//
// exited() should be called *once* per proc, but procd's procclnt may
// call exited() for different procs.
func (clnt *ProcClnt) exited(fsl *fslib.FsLib, procdir string, parentdir string, pid sp.Tpid, status *proc.Status) error {
	db.DPrintf(db.PROCCLNT, "exited %v parent %v pid %v status %v", procdir, parentdir, pid, status)

	// will catch some unintended misuses: a proc calling exited
	// twice or schedd calling exited twice.
	if clnt.setExited(pid) == pid {
		debug.PrintStack()
		db.DFatalf("Exited called after exited %v", procdir)
	}

	return exited(fsl, procdir, parentdir, pid, status)
}

func exited(fsl *fslib.FsLib, procdir string, parentdir string, pid sp.Tpid, status *proc.Status) error {
	b, err := json.Marshal(status)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited marshal err %v", err)
		return err
	}
	// May return an error if parent already exited.
	fn := path.Join(parentdir, proc.EXIT_STATUS)
	if _, err := fsl.PutFile(fn, 0777, sp.OWRITE, b); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited error (parent already exited) MakeFile %v err %v", fn, err)
	}

	semExit := semclnt.MakeSemClnt(fsl, path.Join(procdir, proc.EXIT_SEM))
	if err := semExit.Up(); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited semExit up error: %v, %v, %v", procdir, pid, err)
	}

	// clean myself up
	r := removeProc(fsl, procdir+"/")
	if r != nil {
		return fmt.Errorf("Exited error [%v] %v", procdir, r)
	}

	return nil
}

func (clnt *ProcClnt) Exited(status *proc.Status) {
	err := clnt.exited(clnt.FsLib, clnt.procdir, proc.PARENTDIR, clnt.SigmaConfig().PID, status)
	if err != nil {
		db.DPrintf(db.ALWAYS, "exited %v err %v", clnt.SigmaConfig().PID, err)
	}
}

func ExitedProcd(fsl *fslib.FsLib, pid sp.Tpid, procdir string, parentdir string, status *proc.Status) {
	db.DPrintf(db.PROCCLNT, "exited %v parent %v pid %v status %v", procdir, parentdir, pid, status)
	err := exited(fsl, procdir, parentdir, pid, status)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited %v err %v", pid, err)
	}
	// If proc ran, but crashed before calling Started, the parent may block indefinitely. Stop this from happening by calling semStart.Up()
	semStart := semclnt.MakeSemClnt(fsl, path.Join(parentdir, proc.START_SEM))
	semStart.Up()
}

// ========== EVICT ==========

// Notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) evict(procdir string) error {
	db.DPrintf(db.PROCCLNT, "evict %v", procdir)
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Up()
	if err != nil {
		return fmt.Errorf("Evict error %v", err)
	}
	return nil
}

// Called by parent.
func (clnt *ProcClnt) Evict(pid sp.Tpid) error {
	procdir := proc.GetChildProcDir(clnt.procdir, pid)
	return clnt.evict(procdir)
}

// Called by realm to evict another machine's named.
func (clnt *ProcClnt) EvictKernelProc(pid string) error {
	procdir := path.Join(sp.KPIDSREL, pid)
	return clnt.evict(procdir)
}

// Called by procd.
func (clnt *ProcClnt) EvictProcd(scheddIp string, pid sp.Tpid) error {
	procdir := path.Join(sp.SCHEDD, scheddIp, sp.PIDS, pid.String())
	return clnt.evict(procdir)
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
