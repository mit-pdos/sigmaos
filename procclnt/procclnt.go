package procclnt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

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

type ProcClnt struct {
	sync.Mutex
	*fslib.FsLib
	pid        sp.Tpid
	isExited   sp.Tpid
	procdir    string
	scheddclnt *scheddclnt.ScheddClnt
	cs         *ChildState
}

func newProcClnt(fsl *fslib.FsLib, pid sp.Tpid, procdir string) *ProcClnt {
	clnt := &ProcClnt{
		FsLib:      fsl,
		pid:        pid,
		procdir:    procdir,
		scheddclnt: scheddclnt.NewScheddClnt(fsl),
		cs:         newChildState(),
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

type errTuple struct {
	proc *proc.Proc
	err  error
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

	clnt.cs.spawned(p.GetPid(), kernelId)

	p.InheritParentProcEnv(clnt.ProcEnv())

	// Set the parent dir
	childProcdir := p.GetParentDir()

	db.DPrintf(db.PROCCLNT, "Spawn [%v]: %v", kernelId, p)
	if clnt.hasExited() != "" {
		db.DPrintf(db.PROCCLNT_ERR, "Spawn error called after Exited")
		db.DFatalf("Spawn error called after Exited")
	}

	if spread > 0 {
		// Update the list of active procds.
		clnt.scheddclnt.UpdateSchedds()
		kid, err := clnt.scheddclnt.NextSchedd(spread)
		if err != nil {
			return err
		}
		kernelId = kid
	}

	if err := clnt.addChild(kernelId, p, childProcdir, how); err != nil {
		return err
	}

	p.SetSpawnTime(time.Now())
	// If this is not a privileged proc, spawn it through schedd.
	if how == proc.HSCHEDD {
		if err := clnt.spawnRetry(kernelId, p); err != nil {
			return clnt.cleanupError(p.GetPid(), childProcdir, fmt.Errorf("Spawn error %v", err))
		}
	} else {
		if !isKProc(p.GetPid()) {
			b := debug.Stack()
			db.DFatalf("Tried to Spawn kernel proc %v, stack:\n%v", p.GetPid(), string(b))
		}
		// Make the proc's procdir
		err := clnt.NewProcDir(p.GetPid(), p.GetProcDir(), p.IsPrivileged())
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
	return serr.NewErr(serr.TErrUnreachable, kernelId)
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

// XXX TODO Silly check for priv proc for now. Should Do this in a more
// principle way.
func isKProc(pid sp.Tpid) bool {
	pidstr := pid.String()
	return strings.Contains(pidstr, "named") ||
		strings.Contains(pidstr, "schedd") ||
		strings.Contains(pidstr, "uprocd") ||
		strings.Contains(pidstr, "ux") ||
		strings.Contains(pidstr, "s3") ||
		strings.Contains(pidstr, "realmd") ||
		strings.Contains(pidstr, "db") ||
		strings.Contains(pidstr, "mongo")
}

func (clnt *ProcClnt) waitStart(pid sp.Tpid, how proc.Thow) error {
	db.DPrintf(db.PROCCLNT, "WaitStart %v how %v", pid, how)
	defer db.DPrintf(db.PROCCLNT, "WaitStart done waiting %v how %v", pid, how)

	s := time.Now()
	defer db.DPrintf(db.SPAWN_LAT, "[%v] E2E WaitStart %v", pid, time.Since(s))

	var err error
	// If not a kernel proc...
	if how == proc.HSCHEDD {
		db.DPrintf(db.PROCCLNT, "WaitStart uproc %v", pid)
		// RPC the schedd this proc was spawned on to wait for it to start.
		db.DPrintf(db.ALWAYS, "WaitStart %v RPC pre", pid)
		db.DPrintf(db.PROCCLNT, "WaitStart %v RPC pre", pid)
		kernelID, err := clnt.cs.getKernelID(pid)
		if err != nil {
			db.DFatalf("Unkown kernel ID %v", err)
		}
		rpcc, err := clnt.getScheddClnt(kernelID)
		if err != nil {
			db.DFatalf("Err get schedd clnt rpcc %v", err)
		}
		req := &schedd.StartRequest{
			PidStr: pid.String(),
		}
		res := &schedd.StartResponse{}
		if err := rpcc.RPC("Schedd.WaitStart", req, res); err != nil {
			db.DFatalf("Error Schedd WaitStart: %v", err)
		}
		db.DPrintf(db.PROCCLNT, "WaitStart %v RPC post", pid)
		db.DPrintf(db.ALWAYS, "WaitStart %v RPC post", pid)
		err = nil
	} else {
		kprocDir := proc.KProcDir(pid)
		db.DPrintf(db.PROCCLNT, "WaitStart kproc %v dir %v", pid, kprocDir)
		semStart := semclnt.NewSemClnt(clnt.FsLib, path.Join(kprocDir, proc.START_SEM))
		err = semStart.Down()
	}
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "WaitStart %v %v", pid, err)
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid sp.Tpid) error {
	if isKProc(pid) {
		b := debug.Stack()
		db.DFatalf("Tried to WaitStart kernel proc %v, stack:\n%v", pid, string(b))
	}
	return clnt.waitStart(pid, proc.HSCHEDD)
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStartKernelProc(pid sp.Tpid, how proc.Thow) error {
	return clnt.waitStart(pid, how)
}

func (clnt *ProcClnt) waitExit(pid sp.Tpid, how proc.Thow) (*proc.Status, error) {
	defer clnt.cs.exited(pid)

	// Must wait for child to fill in return status pipe.
	if err := clnt.waitStart(pid, how); err != nil {
		db.DPrintf(db.PROCCLNT, "waitStart err %v", err)
	}

	db.DPrintf(db.PROCCLNT, "WaitExit %v", pid)

	// Make sure the child proc has exited.
	semExit := semclnt.NewSemClnt(clnt.FsLib, path.Join(proc.GetChildProcDir(clnt.procdir, pid), proc.EXIT_SEM))
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

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExit(pid sp.Tpid) (*proc.Status, error) {
	if isKProc(pid) {
		b := debug.Stack()
		db.DFatalf("Tried to WaitExit kernel proc %v, stack:\n%v", pid, string(b))
	}
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
	procdir := clnt.ProcEnv().ProcDir
	db.DPrintf(db.PROCCLNT, "WaitEvict %v procdir %v", pid, procdir)
	defer db.DPrintf(db.PROCCLNT, "WaitEvict done %v", pid)
	semEvict := semclnt.NewSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Down()
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "WaitEvict error %v procdir %v", err, procdir)
		return fmt.Errorf("WaitEvict error %v", err)
	}
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started() error {
	db.DPrintf(db.PROCCLNT, "Started %v", clnt.pid)

	db.DPrintf(db.SPAWN_LAT, "[%v] Proc started %v", clnt.ProcEnv().GetPID(), time.Now())

	// Link self into parent dir
	if err := clnt.linkSelfIntoParentDir(); err != nil {
		db.DPrintf(db.PROCCLNT, "linkSelfIntoParentDir %v err %v", clnt.pid, err)
		return err
	}

	if proc.Thow(clnt.ProcEnv().GetHow()) == proc.HSCHEDD {
		db.DPrintf(db.PROCCLNT, "Started %v RPC pre", clnt.pid)
		db.DPrintf(db.ALWAYS, "Started %v RPC pre", clnt.pid)
		// Get the RPC client for the local schedd
		rpcc, err := clnt.getScheddClnt(clnt.ProcEnv().GetKernelID())
		if err != nil {
			db.DFatalf("Err get schedd clnt rpcc %v", err)
		}
		req := &schedd.StartRequest{
			PidStr: clnt.pid.String(),
		}
		res := &schedd.StartResponse{}
		if err := rpcc.RPC("Schedd.Started", req, res); err != nil {
			db.DFatalf("Error Schedd Started: %v", err)
		}
		db.DPrintf(db.PROCCLNT, "Started %v RPC post", clnt.pid)
		db.DPrintf(db.ALWAYS, "Started %v RPC post", clnt.pid)
		return nil
	} else {
		semPath := path.Join( /*proc.PARENTDIR*/ clnt.ProcEnv().ParentDir, proc.START_SEM)
		if isKProc(clnt.ProcEnv().GetPID()) {
			semPath = path.Join(clnt.ProcEnv().ProcDir, proc.START_SEM)
		}

		// Mark self as started
		semStart := semclnt.NewSemClnt(clnt.FsLib, semPath)
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
		b := debug.Stack()
		db.DFatalf("Exited called after exited %v stack:\n%v", procdir, string(b))
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
		db.DPrintf(db.PROCCLNT_ERR, "exited error (parent already exited) NewFile %v err %v", fn, err)
	}

	semExit := semclnt.NewSemClnt(fsl, path.Join(procdir, proc.EXIT_SEM))
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
	err := clnt.exited(clnt.FsLib, clnt.procdir /*proc.PARENTDIR*/, clnt.ProcEnv().ParentDir, clnt.ProcEnv().GetPID(), status)
	if err != nil {
		db.DPrintf(db.ALWAYS, "exited %v err %v", clnt.ProcEnv().GetPID(), err)
	}
}

func ExitedProcd(fsl *fslib.FsLib, pid sp.Tpid, procdir string, parentdir string, status *proc.Status, how proc.Thow) {
	db.DPrintf(db.PROCCLNT, "exited %v parent %v pid %v status %v", procdir, parentdir, pid, status)
	err := exited(fsl, procdir, parentdir, pid, status)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited %v err %v", pid, err)
	}
	// If proc ran, but crashed before calling Started, the parent may block indefinitely. Stop this from happening by calling semStart.Up()
	semPath := path.Join(parentdir, proc.START_SEM)
	if how != proc.HSCHEDD {
		kprocDir := proc.KProcDir(pid)
		semPath = path.Join(kprocDir, proc.START_SEM)
	}
	semStart := semclnt.NewSemClnt(fsl, semPath)
	semStart.Up()
}

// ========== EVICT ==========

// Notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) evict(procdir string) error {
	db.DPrintf(db.PROCCLNT, "Evict %v", procdir)
	semEvict := semclnt.NewSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
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
