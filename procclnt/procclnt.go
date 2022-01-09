package procclnt

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
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
	// name for dir where procs live. May not refer to name/pids
	// because proc.PidDir may change it.  A proc refers to itself
	// using "pids/<pid>", where pid is the proc's PID.
	PIDS = "pids"

	// Files/directories in "pids/<pid>":
	START_WAIT  = "start-sem"
	EVICT_WAIT  = "evict-sem"
	RET_STATUS  = "status-pipe"
	CHILD       = "childs"      // directory with children's pids
	KERNEL_PROC = "kernel-proc" // Only present if this is a kernel proc

	MAXSTATUS = 1000
)

type ProcClnt struct {
	mu sync.Mutex
	*fslib.FsLib
	pid      string
	piddir   string
	isExited string
}

func makeProcClnt(fsl *fslib.FsLib, piddir, pid string) *ProcClnt {
	clnt := &ProcClnt{}
	clnt.FsLib = fsl
	clnt.pid = pid
	clnt.piddir = piddir
	return clnt
}

// ========== SPAWN ==========

// XXX Should probably eventually fold this into spawn (but for now, we may want to get the exec.Cmd struct back).
func (clnt *ProcClnt) SpawnKernelProc(p *proc.Proc, bin string, namedAddr []string) (*exec.Cmd, error) {
	if err := clnt.Spawn(p); err != nil {
		return nil, err
	}
	return proc.Run(p.Pid, bin, p.Program, namedAddr, p.Args)
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {

	// Select which queue to put the job in
	piddir := proc.PidDir(p.Pid)

	// log.Printf("%v: %p spawn %v\n", db.GetName(), clnt, piddir)

	if clnt.hasExited() != "" {
		return fmt.Errorf("Spawn error called after Exited")
	}

	if err := clnt.Mkdir(piddir, 0777); err != nil {
		log.Printf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), piddir, err)
		return err
	}
	if clnt.piddir != p.PidDir {
		log.Printf("%v: spawn child %v make piddir %v\n", db.GetName(), clnt.piddir, p.PidDir)
		if err := clnt.Mkdir(p.PidDir, 0777); err != nil {
			log.Printf("%v: Spawn new piddir %v err %v\n", db.GetName(), p.PidDir, err)
			return clnt.cleanupError(piddir, fmt.Errorf("Spawn error %v", err))
		}
		piddir = p.PidDir + "/" + p.Pid
		if err := clnt.Mkdir(piddir, 0777); err != nil {
			log.Printf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), piddir, err)
			return clnt.cleanupError(piddir, fmt.Errorf("Spawn error %v", err))
		}
	}

	err := clnt.MakePipe(piddir+"/"+RET_STATUS, 0777)
	if err != nil {
		log.Printf("%v: MakePipe %v err %v\n", db.GetName(), RET_STATUS, err)
		return clnt.cleanupError(piddir, fmt.Errorf("Spawn error %v", err))
	}

	semStart := semclnt.MakeSemClnt(clnt.FsLib, piddir+"/"+START_WAIT)
	semStart.Init()

	semEvict := semclnt.MakeSemClnt(clnt.FsLib, piddir+"/"+EVICT_WAIT)
	semEvict.Init()

	d := piddir + "/" + CHILD
	if err := clnt.Mkdir(d, 0777); err != nil {
		log.Printf("%v: Spawn mkdir childs %v err %v\n", db.GetName(), d, err)
		return clnt.cleanupError(piddir, fmt.Errorf("Spawn error %v", err))
	}

	// Add pid to my children
	f := PIDS + "/" + clnt.pid + "/" + CHILD + "/" + p.Pid
	if err := clnt.MakeFile(f, 0777, np.OWRITE, []byte{}); err != nil {
		log.Printf("%v: Spawn mkfile child %v err %v\n", db.GetName(), f, err)
		return clnt.cleanupError(piddir, err)
	}

	// If this is not a kernel proc, spawn it through procd.
	if !p.IsKernelProc() && !p.IsRealmProc() {
		b, err := json.Marshal(p)
		if err != nil {
			log.Printf("%v: marshal err %v", db.GetName(), err)
			return clnt.cleanupError(piddir, fmt.Errorf("Spawn error %v", err))
		}
		fn := path.Join(np.PROCDREL+"/~ip", np.PROC_CTL_FILE)
		err = clnt.WriteFile(fn, b)
		if err != nil {
			log.Printf("%v: WriteFile %v err %v", db.GetName(), fn, err)
			return clnt.cleanupError(piddir, fmt.Errorf("Spawn error %v", err))
		}
	} else {
		fn := path.Join(piddir, KERNEL_PROC)
		if err := clnt.MakeFile(fn, 0777, np.OWRITE, []byte{}); err != nil {
			log.Printf("%v: MakeFile %v err %v", db.GetName(), fn, err)
			return clnt.cleanupError(piddir, fmt.Errorf("Spawn error %v", err))
		}
	}

	return nil
}

// ========== WAIT ==========

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid string) error {
	piddir := proc.PidDir(pid)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, piddir+"/"+START_WAIT)
	err := semStart.Down()
	if err != nil {
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitExited() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent cleans up the child and parent removes the
// child from its list of children.
func (clnt *ProcClnt) WaitExit(pid string) (string, error) {
	piddir := proc.PidDir(pid)

	// log.Printf("%v: %p waitexit %v\n", db.GetName(), clnt, piddir)

	if _, err := clnt.Stat(piddir); err != nil {
		return "", fmt.Errorf("WaitExit error %v", err)
	}

	fn := piddir + "/" + RET_STATUS
	fd, err := clnt.Open(piddir+"/"+RET_STATUS, np.OREAD)
	if err != nil {
		log.Printf("%v: Open %v err %v", db.GetName(), fn, err)
		return "", fmt.Errorf("WaitExit error %v", err)
	}

	b, err := clnt.Read(fd, MAXSTATUS)
	if err != nil {
		log.Printf("Read %v err %v", fn, err)
		return "", fmt.Errorf("WaitExit error %v", err)
	}

	err = clnt.Close(fd)
	if err != nil {
		log.Printf("Close %v err %v", fn, err)
		return "", fmt.Errorf("WaitExit error %v", err)
	}

	// Remove pid from my children now its status has been
	// collected We don't need to abandon it.
	f := PIDS + "/" + clnt.pid + "/" + CHILD + "/" + path.Base(pid)
	if err := clnt.Remove(f); err != nil {
		log.Printf("Error Remove %v in WaitExit: %v", f, err)
		return "", fmt.Errorf("WaitExit error %v", err)
	}

	// XXX what happens if we crash here; who will collect? procd?
	clnt.removeProc(piddir)

	return string(b), nil

}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid string) error {
	piddir := proc.PidDir(pid)
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, piddir+"/"+EVICT_WAIT)
	err := semEvict.Down()
	if err != nil {
		return fmt.Errorf("WaitEvict error %v", err)
	}
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started(pid string) error {
	dir := proc.PidDir(pid)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, dir+"/"+START_WAIT)
	err := semStart.Up()
	if err != nil {
		return fmt.Errorf("Started error %v", err)
	}
	// Only isolate kernel procs
	if !clnt.isKernelProc(pid) {
		// Isolate the process namespace
		newRoot := os.Getenv("NEWROOT")
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
// have.  If itself is an abandoned child, then it cleans up itself;
// otherwise the parent will do it.
//
// exited() should be called *once* per proc, but procd's procclnt may
// call exited() for different procs.
func (clnt *ProcClnt) exited(pid string, status string) error {
	piddir := proc.PidDir(pid)

	// will catch some unintended misuses: a proc calling exited
	// twice or procd calling exited twice.
	if clnt.setExited(pid) == pid {
		log.Printf("%v: Exited called after exited %v\n", db.GetName(), piddir)
		return fmt.Errorf("Exited error called more than once for pid %v", pid)
	}

	// log.Printf("%v: exited %v\n", db.GetName(), piddir)

	// Abandon any children I may have left.
	err := clnt.abandonChildren(pid)
	if err != nil {
		return fmt.Errorf("Exited error %v", err)
	}

	fn := piddir + "/" + RET_STATUS
	fd, err := clnt.Open(fn, np.OWRITE)
	if err != nil {
		// parent has abandoned me; clean myself up
		// log.Printf("%v: Error Open %v err %v", db.GetName(), fn, err)
		r := clnt.removeProc(piddir)
		if r != nil {
			return fmt.Errorf("Exited error %v", r)
		}
	} else {
		_, err = clnt.Write(fd, []byte(status))
		if err != nil {
			log.Printf("Write %v err %v", fn, err)
			return fmt.Errorf("Exited error %v", err)
		}
		err = clnt.Close(fd)
		if err != nil {
			log.Printf("Close %v err %v", fn, err)
			return fmt.Errorf("Exited error %v", err)
		}
	}

	return nil
}

// If exited() fails, invoke os.Exit(1) to indicate to procd that proc
// failed
func (clnt *ProcClnt) Exited(pid string, status string) {
	err := clnt.exited(pid, status)
	if err != nil {
		log.Printf("%v: exited %v err %v\n", db.GetName(), pid, err)
		os.Exit(1)
	}
}

func (clnt *ProcClnt) ExitedErr(pid string, status string) error {
	err := clnt.exited(pid, status)
	if err != nil {
		log.Printf("%v: exitederr %v err %v\n", db.GetName(), pid, err)
	}
	return err
}

func (clnt *ProcClnt) ExitedProcd(pid string, status string) {
	err := clnt.exited(pid, status)
	if err != nil {
		// XXX maybe remove any state left of proc?
		log.Printf("%v: exited %v err %v\n", db.GetName(), pid, err)
	}
}

// ========== EVICT ==========

// Procd notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) Evict(pid string) error {
	piddir := proc.PidDir(pid)
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, piddir+"/"+EVICT_WAIT)
	err := semEvict.Up()
	if err != nil {
		return fmt.Errorf("Evict error %v", err)
	}
	return nil
}

// ========== GETCHILDREN ==========

// Return the pids of all children.
func (clnt *ProcClnt) GetChildren(pid string) ([]string, error) {
	piddir := proc.PidDir(pid)
	sts, err := clnt.ReadDir(path.Join(piddir, CHILD))
	if err != nil {
		log.Printf("%v: GetChildren %v error: %v", db.GetName(), pid, err)
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
func (clnt *ProcClnt) abandonChildren(pid string) error {
	cpids, err := clnt.GetChildren(pid)
	if err != nil {
		log.Printf("%v: abandonChildren  %v error: %v", db.GetName(), pid, err)
		return err
	}

	for _, cpid := range cpids {
		r := clnt.abandonChild(PIDS + "/" + cpid)
		if r != nil && err != nil {
			err = r
		}
	}
	return err
}

// Abandon child
func (clnt *ProcClnt) abandonChild(piddir string) error {
	f := piddir + "/" + RET_STATUS
	err := clnt.Remove(f)
	if err != nil {
		log.Printf("%v: Remove %v err %v\n", db.GetName(), f, err)
		return err
	}
	return nil
}

// Clean up proc
func (clnt *ProcClnt) removeProc(piddir string) error {
	// log.Printf("%v: removeProc %v\n", db.GetName(), piddir)
	if err := clnt.RmDir(piddir); err != nil {
		s, _ := clnt.SprintfDir(piddir)
		log.Printf("%v: RmDir %v err %v %v", db.GetName(), piddir, err, s)
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

// Attempt to cleanup piddir
func (clnt *ProcClnt) cleanupError(piddir string, err error) error {
	clnt.removeProc(piddir)
	return err
}

func (clnt *ProcClnt) isKernelProc(pid string) bool {
	piddir := proc.PidDir(pid)
	_, err := clnt.Stat(path.Join(piddir, KERNEL_PROC))
	return err == nil
}
