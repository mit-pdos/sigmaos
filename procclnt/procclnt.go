package procclnt

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime/debug"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/seccomp"
	"ulambda/semclnt"
)

const (
	MAXSTATUS = 1000
)

type ProcClnt struct {
	mu sync.Mutex
	*fslib.FsLib
	pid      string
	isExited string
}

func makeProcClnt(fsl *fslib.FsLib, pid string) *ProcClnt {
	clnt := &ProcClnt{}
	clnt.FsLib = fsl
	clnt.pid = pid
	return clnt
}

// ========== SPAWN ==========

// XXX Should probably eventually fold this into spawn (but for now, we may want to get the exec.Cmd struct back).
func (clnt *ProcClnt) SpawnKernelProc(p *proc.Proc, bin string, namedAddr []string) (*exec.Cmd, error) {
	if err := clnt.Spawn(p); err != nil {
		return nil, err
	}

	// Make the proc's procdir
	err := clnt.MakeProcDir(p.Pid, p.ProcDir, p.IsPrivilegedProc())
	if err != nil {
		log.Printf("Err SpawnKernelProc MakeProcDir: %v", err)
	}

	return proc.RunKernelProc(p, bin, namedAddr)
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	procdir := p.ProcDir

	// log.Printf("%v: %p spawn %v\n", db.GetName(), clnt, procdir)

	if clnt.hasExited() != "" {
		return fmt.Errorf("Spawn error called after Exited")
	}

	if err := clnt.addChild(p.Pid, procdir); err != nil {
		return err
	}

	err := clnt.MakePipe(proc.GetChildStatusPath(p.Pid), 0777)
	if err != nil {
		log.Printf("%v: MakePipe %v err %v\n", db.GetName(), proc.RET_STATUS, err)
		return clnt.cleanupError(p.Pid, proc.GetChildStatusPath(p.Pid), fmt.Errorf("Spawn error %v", err))
	}

	// Create a semaphore to indicate a proc has started.
	childDir := path.Dir(proc.GetChildProcDir(p.Pid))
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
	semStart.Init()

	// If this is not a privileged proc, spawn it through procd.
	if !p.IsPrivilegedProc() {
		b, err := json.Marshal(p)
		if err != nil {
			log.Printf("%v: marshal err %v", db.GetName(), err)
			return clnt.cleanupError(p.Pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
		fn := path.Join(np.PROCDREL+"/~ip", np.PROC_CTL_FILE)
		err = clnt.WriteFile(fn, b)
		if err != nil {
			log.Printf("%v: WriteFile %v err %v", db.GetName(), fn, err)
			return clnt.cleanupError(p.Pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
	}

	return nil
}

// ========== WAIT ==========

func (clnt *ProcClnt) waitStart(pid string) error {
	childDir := path.Dir(proc.GetChildProcDir(pid))
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
	err := semStart.Down()
	return err
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid string) error {
	err := clnt.waitStart(pid)
	if err != nil {
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent removes the child from its list of children.
func (clnt *ProcClnt) WaitExit(pid string) (string, error) {
	childDir := path.Dir(proc.GetChildProcDir(pid))
	// log.Printf("%v: %p waitexit %v\n", db.GetName(), clnt, procdir)

	if _, err := clnt.Stat(childDir); err != nil {
		return "", fmt.Errorf("WaitExit error 1 %v %v", err, childDir)
	}

	pipePath := proc.GetChildStatusPath(pid)
	fd, err := clnt.Open(pipePath, np.OREAD)
	if err != nil {
		log.Printf("%v: Open %v err %v", db.GetName(), pipePath, err)
		return "", fmt.Errorf("WaitExit error 2 %v", err)
	}

	b, err := clnt.Read(fd, MAXSTATUS)
	if err != nil {
		log.Printf("Read %v err %v", pipePath, err)
		return "", fmt.Errorf("WaitExit error 3 %v", err)
	}

	err = clnt.Close(fd)
	if err != nil {
		log.Printf("Close %v err %v", pipePath, err)
		return "", fmt.Errorf("WaitExit error 4 %v", err)
	}

	clnt.removeChild(pid)

	return string(b), nil
}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid string) error {
	procdir := proc.PROCDIR
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Down()
	if err != nil {
		return fmt.Errorf("WaitEvict error %v", err)
	}
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started(pid string) error {
	procdir := proc.PROCDIR

	// Link self into parent dir
	if err := clnt.linkChildIntoParentDir(pid, procdir); err != nil {
		return err
	}

	// Create eviction signal
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	semEvict.Init()

	// Mark self as started
	parentDir := proc.PARENTDIR
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(parentDir, proc.START_SEM))
	err := semStart.Up()
	// File may not be found if parent exited first.
	if err != nil && err.Error() != "file not found" {
		log.Printf("Started error %v %v", path.Join(parentDir, proc.START_SEM), err)
		return fmt.Errorf("Started error %v", err)
	}
	// Only isolate kernel procs
	if !clnt.isKernelProc(pid) {
		// Isolate the process namespace
		newRoot := proc.GetNewRoot()
		if err := namespace.Isolate(newRoot); err != nil {
			log.Printf("Error Isolate in clnt.Started: %v", err)
			return fmt.Errorf("Started error %v", err)
		}
		// Load a seccomp filter.
		seccomp.LoadFilter()
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
func (clnt *ProcClnt) exited(procdir string, parentdir string, pid string, status string) error {
	// will catch some unintended misuses: a proc calling exited
	// twice or procd calling exited twice.
	if clnt.setExited(pid) == pid {
		log.Printf("%v: Exited called after exited %v\n", db.GetName(), procdir)
		return fmt.Errorf("Exited error called more than once for pid %v", pid)
	}

	// Abandon any children I may have left.
	err := clnt.abandonChildren(procdir)
	if err != nil {
		return fmt.Errorf("Exited error %v", err)
	}

	// clean myself up
	r := clnt.removeProc(procdir)
	if r != nil {
		return fmt.Errorf("Exited error %v", r)
	}

	pipePath := path.Join(parentdir, proc.RET_STATUS)
	fd, err := clnt.Open(pipePath, np.OWRITE)
	if err == nil {
		_, err = clnt.Write(fd, []byte(status))
		if err != nil {
			log.Printf("Write %v err %v", pipePath, err)
			return fmt.Errorf("Exited error %v", err)
		}
		err = clnt.Close(fd)
		if err != nil {
			log.Printf("Close %v err %v", pipePath, err)
			return fmt.Errorf("Exited error %v", err)
		}
	}

	return nil
}

// If exited() fails, invoke os.Exit(1) to indicate to procd that proc
// failed
func (clnt *ProcClnt) Exited(pid string, status string) {
	procdir := proc.PROCDIR
	err := clnt.exited(procdir, proc.PARENTDIR, pid, status)
	if err != nil {
		log.Printf("%v: exited %v err %v\n", db.GetName(), pid, err)
		os.Exit(1)
	}
}

func (clnt *ProcClnt) ExitedProcd(pid string, parentdir string, status string) {
	procdir := path.Join(proc.PIDS, pid)
	err := clnt.exited(procdir, parentdir, pid, status)
	if err != nil {
		// XXX maybe remove any state left of proc?
		log.Printf("%v: exited %v err %v\n", db.GetName(), pid, err)
	}
}

// ========== EVICT ==========

// Notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) evict(procdir string) error {
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Up()
	if err != nil {
		return fmt.Errorf("Evict error %v", err)
	}
	return nil
}

// Called by parent.
func (clnt *ProcClnt) Evict(pid string) error {
	procdir := proc.GetChildProcDir(pid)
	return clnt.evict(procdir)
}

// Called by procd.
func (clnt *ProcClnt) EvictProcd(pid string) error {
	procdir := path.Join(proc.PIDS, pid)
	return clnt.evict(procdir)
}

// ========== GETCHILDREN ==========

// Return the pids of all children.
func (clnt *ProcClnt) GetChildren(procdir string) ([]string, error) {
	sts, err := clnt.ReadDir(path.Join(procdir, proc.CHILDREN))
	if err != nil {
		log.Printf("%v: GetChildren %v error: %v", db.GetName(), procdir, err)
		return nil, err
	}
	cpids := []string{}
	for _, st := range sts {
		cpids = append(cpids, st.Name)
	}
	return cpids, nil
}

// ========== Helpers ==========

// Remove status from children to indicate we don't care
func (clnt *ProcClnt) abandonChildren(procdir string) error {
	cpids, err := clnt.GetChildren(procdir)
	if err != nil {
		log.Printf("%v: abandonChildren  %v error: %v", db.GetName(), procdir, err)
		return err
	}

	for _, cpid := range cpids {
		r := clnt.abandonChild(cpid)
		if r != nil && err != nil {
			err = r
		}
	}
	return err
}

// Abandon child
func (clnt *ProcClnt) abandonChild(pid string) error {
	f := proc.GetChildStatusPath(pid)
	err := clnt.Remove(f)
	if err != nil {
		log.Printf("%v: Remove %v err %v\n", db.GetName(), f, err)
		return err
	}
	return nil
}

// Clean up proc
func (clnt *ProcClnt) removeProc(procdir string) error {
	// log.Printf("%v: removeProc %v\n", db.GetName(), procdir)
	if err := clnt.RmDir(procdir); err != nil {
		s, _ := clnt.SprintfDir(procdir)
		debug.PrintStack()
		log.Printf("%v: RmDir %v err %v \n%v", db.GetName(), procdir, err, s)
		return err
	}
	return nil
}

func (clnt *ProcClnt) hasExited() string {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()
	return clnt.isExited
}

func (clnt *ProcClnt) setExited(pid string) string {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()
	r := clnt.isExited
	clnt.isExited = pid
	return r
}
