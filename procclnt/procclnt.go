package procclnt

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/proc"
	// "ulambda/seccomp"
	"ulambda/semclnt"
)

const (
	MAXSTATUS = 1000
)

type ProcClnt struct {
	mu sync.Mutex
	*fslib.FsLib
	pid      proc.Tpid
	isExited proc.Tpid
}

func makeProcClnt(fsl *fslib.FsLib, pid proc.Tpid) *ProcClnt {
	clnt := &ProcClnt{}
	clnt.FsLib = fsl
	clnt.pid = pid
	return clnt
}

// ========== SPAWN ==========

// XXX Should probably eventually fold this into spawn (but for now, we may want to get the exec.Cmd struct back).
func (clnt *ProcClnt) SpawnKernelProc(p *proc.Proc, bindir string, namedAddr []string) (*exec.Cmd, error) {
	if err := clnt.Spawn(p); err != nil {
		return nil, err
	}

	// Make the proc's procdir
	err := clnt.MakeProcDir(p.Pid, p.ProcDir, p.IsPrivilegedProc())
	if err != nil {
		db.DLPrintf("PROCCLNT_ERR", "Err SpawnKernelProc MakeProcDir: %v", err)
	}

	return proc.RunKernelProc(p, bindir, namedAddr)
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	procdir := p.ProcDir

	db.DLPrintf("PROCCLNT", "Spawn %v\n", p)
	if clnt.hasExited() != "" {
		db.DLPrintf("PROCCLNT_ERR", "Spawn error called after Exited")
		os.Exit(0)
	}

	if err := clnt.addChild(p.Pid, procdir, p.GetShared()); err != nil {
		return err
	}

	// If this is not a privileged proc, spawn it through procd.
	if !p.IsPrivilegedProc() {
		b, err := json.Marshal(p)
		if err != nil {
			db.DLPrintf("PROCLNT_ERR", "Spawn marshal err %v\n", err)
			return clnt.cleanupError(p.Pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
		fn := path.Join(np.PROCDREL+"/~ip", np.PROC_CTL_FILE)
		_, err = clnt.SetFile(fn, b, np.OWRITE, 0)
		if err != nil {
			db.DLPrintf("PROCCLNT_ERR", "SetFile %v err %v", fn, err)
			return clnt.cleanupError(p.Pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
	} else {
		// Create a semaphore to indicate a proc has started if this is a kernel
		// proc. Otherwise, procd will create the semaphore.
		childDir := path.Dir(proc.GetChildProcDir(p.Pid))
		semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
		semStart.Init(0)
	}

	return nil
}

// ========== WAIT ==========

func (clnt *ProcClnt) waitStart(pid proc.Tpid) error {
	childDir := path.Dir(proc.GetChildProcDir(pid))
	db.DLPrintf("PROCCLNT", "WaitStart %v %v\n", pid, childDir)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
	err := semStart.Down()
	return err
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid proc.Tpid) error {
	err := clnt.waitStart(pid)
	if err != nil {
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExit(pid proc.Tpid) (*proc.Status, error) {
	// Must wait for child to fill in return status pipe.
	if err := clnt.waitStart(pid); err != nil {
		db.DLPrintf("PROCCLNT", "waitStarted err %v\n", err)
	}

	db.DLPrintf("PROCCLNT", "WaitExit %v\n", pid)

	// Make sure the child proc has exited.
	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(proc.GetChildProcDir(pid), proc.EXIT_SEM))
	if err := semExit.Down(); err != nil {
		db.DLPrintf("PROCCLNT", "Error WaitExit semExit.Down: %v", err)
		return nil, fmt.Errorf("Error semExit.Down: %v", err)
	}

	childDir := path.Dir(proc.GetChildProcDir(pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.EXIT_STATUS))
	if err != nil {
		db.DLPrintf("PROCCLNT_ERR", "Missing return status, procd must have crashed: %v, %v\n", pid, err)
		return nil, fmt.Errorf("Missing return status, procd must have crashed: %v", err)
	}

	clnt.removeChild(pid)

	status := &proc.Status{}
	if err := json.Unmarshal(b, status); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "waitexit unmarshal err %v", err)
		return nil, err
	}

	return status, nil
}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid proc.Tpid) error {
	db.DLPrintf("PROCCLNT", "WaitEvict %v\n", pid)
	procdir := proc.PROCDIR
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Down()
	if err != nil {
		return fmt.Errorf("WaitEvict error %v", err)
	}
	db.DLPrintf("PROCCLNT", "WaitEvict evicted %v\n", pid)
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started() error {
	pid := proc.GetPid()
	procdir := proc.PROCDIR

	db.DLPrintf("PROCCLNT", "Started %v\n", pid)

	// Link self into parent dir
	if err := clnt.linkChildIntoParentDir(pid, procdir); err != nil {
		db.DLPrintf("PROCCLNT", "linkChildIntoParentDir %v err %v\n", pid, err)
		return err
	}

	// Mark self as started
	parentDir := proc.PARENTDIR
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(parentDir, proc.START_SEM))
	err := semStart.Up()
	if err != nil {
		db.DLPrintf("PROCCLNT_ERR", "Started error %v %v\n", path.Join(parentDir, proc.START_SEM), err)
	}
	// File may not be found if parent exited first or isn't reachable
	if err != nil && !np.IsErrUnavailable(err) {
		return fmt.Errorf("Started error %v", err)
	}

	// Only isolate kernel procs
	if !clnt.isKernelProc(pid) {
		// Isolate the process namespace
		newRoot := proc.GetNewRoot()
		if err := namespace.Isolate(newRoot); err != nil {
			db.DLPrintf("PROCCLNT_ERR", "Error Isolate in clnt.Started: %v\n", err)
			return fmt.Errorf("Started error %v", err)
		}
		// Load a seccomp filter.
		// seccomp.LoadFilter()
	}
	return nil
}

// ========== EXITED ==========

// Proc pid mark itself as exited. Typically exited() is called by
// proc pid, but if the proc crashes, procd calls exited() on behalf
// of the failed proc. The exited proc abandons any chidren it may
// have. The exited proc cleans itself up.
//
// exited() should be called *once* per proc, but procd's procclnt may
// call exited() for different procs.
func (clnt *ProcClnt) exited(procdir string, parentdir string, pid proc.Tpid, status *proc.Status) error {
	db.DLPrintf("PROCCLNT", "exited %v parent %v pid %v status %v\n", procdir, parentdir, pid, status)

	// will catch some unintended misuses: a proc calling exited
	// twice or procd calling exited twice.
	if clnt.setExited(pid) == pid {
		db.DLPrintf("PROCCLNT_ERR", "Exited called after exited %v\n", procdir)
		os.Exit(1)
	}

	b, err := json.Marshal(status)
	if err != nil {
		db.DLPrintf("PROCCLNT_ERR", "exited marshal err %v", err)
		return err
	}
	// May return an error if parent already exited.
	fn := path.Join(parentdir, proc.EXIT_STATUS)
	if _, err := clnt.PutFile(fn, 0777, np.OWRITE, b); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "exited error (parent already exited) MakeFile %v err %v\n", fn, err)
	}

	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EXIT_SEM))
	if err := semExit.Up(); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "exited semExit up error: %v, %v, %v\n", procdir, pid, err)
	}

	// clean myself up
	r := clnt.removeProc(procdir)
	if r != nil {
		return fmt.Errorf("Exited error %v", r)
	}

	return nil
}

// If exited() fails, invoke os.Exit(1) to indicate to procd that proc
// failed
func (clnt *ProcClnt) Exited(status *proc.Status) {
	procdir := proc.PROCDIR
	err := clnt.exited(procdir, proc.PARENTDIR, proc.GetPid(), status)
	if err != nil {
		db.DLPrintf("PROCCLNT_ERR", "exited %v err %v\n", proc.GetPid(), err)
		os.Exit(1)
	}
}

func (clnt *ProcClnt) ExitedProcd(pid proc.Tpid, procdir string, parentdir string, status *proc.Status) {
	db.DLPrintf("PROCCLNT", "exited %v parent %v pid %v status %v\n", procdir, parentdir, pid, status)
	err := clnt.exited(procdir, parentdir, pid, status)
	if err != nil {
		// XXX maybe remove any state left of proc?
		db.DLPrintf("PROCCLNT_ERR", "exited %v err %v\n", pid, err)
	}
	// If proc ran, but crashed before calling Started, the parent may block indefinitely. Stop this from happening by calling semStart.Up()
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(parentdir, proc.START_SEM))
	semStart.Up()
}

// ========== EVICT ==========

// Notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) evict(procdir string) error {
	db.DLPrintf("PROCCLNT", "evict %v\n", procdir)
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Up()
	if err != nil {
		return fmt.Errorf("Evict error %v", err)
	}
	return nil
}

// Called by parent.
func (clnt *ProcClnt) Evict(pid proc.Tpid) error {
	procdir := proc.GetChildProcDir(pid)
	return clnt.evict(procdir)
}

// Called by realm to evict another machine's named.
func (clnt *ProcClnt) EvictKernelProc(pid string) error {
	procdir := path.Join(proc.KPIDS, pid)
	return clnt.evict(procdir)
}

// Called by procd.
func (clnt *ProcClnt) EvictProcd(procdIp string, pid proc.Tpid) error {
	procdir := path.Join(np.PROCD, procdIp, proc.PIDS, pid.String())
	return clnt.evict(procdir)
}

// ========== GETCHILDREN ==========

// Return the pids of all children.
func (clnt *ProcClnt) GetChildren(procdir string) ([]proc.Tpid, error) {
	sts, err := clnt.GetDir(path.Join(procdir, proc.CHILDREN))
	if err != nil {
		db.DLPrintf("PROCCLNT_ERR", "GetChildren %v error: %v", procdir, err)
		return nil, err
	}
	cpids := []proc.Tpid{}
	for _, st := range sts {
		cpids = append(cpids, proc.Tpid(st.Name))
	}
	return cpids, nil
}

// ========== Helpers ==========

// Clean up proc
func (clnt *ProcClnt) removeProc(procdir string) error {
	// Children may try to write in symlinks & exit statuses while the rmdir is
	// happening. In order to avoid causing errors (such as removing a non-empty
	// dir) temporarily rename so children can't find the dir. The dir may be
	// missing already if a proc died while exiting, and this is a procd trying
	// to exit on its behalf.
	src := path.Join(procdir, proc.CHILDREN)
	dst := path.Join(procdir, ".tmp."+proc.CHILDREN)
	if err := clnt.Rename(src, dst); err != nil {
		db.DLPrintf("PROCCLNT_ERR", "Error rename removeProc %v -> %v : %v\n", src, dst, err)
	}
	err := clnt.RmDir(procdir)
	maxRetries := 2
	// May have to retry a few times if writing child already opened dir. We
	// should only have to retry once at most.
	for i := 0; i < maxRetries && err != nil; i++ {
		s, _ := clnt.SprintfDir(procdir)
		// debug.PrintStack()
		db.DLPrintf("PROCCLNT_ERR", "RmDir %v err %v \n%v", procdir, err, s)
		// Retry
		err = clnt.RmDir(procdir)
	}
	return err
}

func (clnt *ProcClnt) hasExited() proc.Tpid {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()
	return clnt.isExited
}

func (clnt *ProcClnt) setExited(pid proc.Tpid) proc.Tpid {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()
	r := clnt.isExited
	clnt.isExited = pid
	return r
}
