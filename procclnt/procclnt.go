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

	return proc.RunKernelProc(p, bin, namedAddr)
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	procdir := p.ProcDir

	// log.Printf("%v: %p spawn %v\n", db.GetName(), clnt, procdir)

	if clnt.hasExited() != "" {
		return fmt.Errorf("Spawn error called after Exited")
	}

	if err := clnt.Mkdir(procdir, 0777); err != nil {
		log.Printf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), procdir, err)
		return err
	}
	//	if clnt.procdir != p.PidDir {
	//		log.Printf("%v: spawn child %v make procdir %v\n", db.GetName(), clnt.procdir, p.PidDir)
	//		if err := clnt.Mkdir(p.PidDir, 0777); err != nil {
	//			log.Printf("%v: Spawn new procdir %v err %v\n", db.GetName(), p.PidDir, err)
	//			return clnt.cleanupError(procdir, fmt.Errorf("Spawn error %v", err))
	//		}
	//		procdir = p.PidDir + "/" + p.Pid
	//		if err := clnt.Mkdir(procdir, 0777); err != nil {
	//			log.Printf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), procdir, err)
	//			return clnt.cleanupError(procdir, fmt.Errorf("Spawn error %v", err))
	//		}
	//	}

	err := clnt.MakePipe(path.Join(procdir, proc.RET_STATUS), 0777)
	if err != nil {
		log.Printf("%v: MakePipe %v err %v\n", db.GetName(), proc.RET_STATUS, err)
		return clnt.cleanupError(p.Pid, procdir, fmt.Errorf("Spawn error %v", err))
	}

	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.START_SEM))
	semStart.Init()

	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	semEvict.Init()

	childDir := path.Join(procdir, proc.CHILDREN)
	if err := clnt.Mkdir(childDir, 0777); err != nil {
		log.Printf("%v: Spawn mkdir childs %v err %v\n", db.GetName(), childDir, err)
		return clnt.cleanupError(p.Pid, procdir, fmt.Errorf("Spawn error %v", err))
	}

	// Add symlink to child
	link := path.Join(proc.PIDS, clnt.pid, proc.CHILDREN, p.Pid)
	if err := clnt.Symlink([]byte(procdir), link, 0777); err != nil {
		log.Printf("%v: Spawn Symlink child %v err %v\n", db.GetName(), link, err)
		return clnt.cleanupError(p.Pid, procdir, err)
	}

	//	log.Printf("Spawning %v, expected len %v, symlink len %v")

	// If this is not a kernel proc, spawn it through procd.
	if !p.IsKernelProc() && !p.IsRealmProc() {
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
	} else {
		fn := path.Join(procdir, proc.KERNEL_PROC)
		if err := clnt.MakeFile(fn, 0777, np.OWRITE, []byte{}); err != nil {
			log.Printf("%v: MakeFile %v err %v", db.GetName(), fn, err)
			return clnt.cleanupError(p.Pid, procdir, fmt.Errorf("Spawn error %v", err))
		}
	}

	return nil
}

// ========== WAIT ==========

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid string) error {
	procdir := path.Join(proc.GetProcDir(), proc.CHILDREN, pid)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.START_SEM))
	err := semStart.Down()
	if err != nil {
		return fmt.Errorf("WaitStart error %v", err)
	}
	return nil
}

// Parent calls WaitExit() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent cleans up the child and parent removes the
// child from its list of children.
func (clnt *ProcClnt) WaitExit(pid string) (string, error) {
	procdir := path.Join(proc.GetProcDir(), proc.CHILDREN, pid)

	// log.Printf("%v: %p waitexit %v\n", db.GetName(), clnt, procdir)

	if _, err := clnt.Stat(procdir); err != nil {
		return "", fmt.Errorf("WaitExit error 1 %v %v", err, procdir)
	}

	pipePath := path.Join(procdir, proc.RET_STATUS)
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

	// Read the child link
	b1, err := clnt.ReadFile(procdir)
	if err != nil {
		log.Printf("Read link err %v %v", procdir, err)
		return "", fmt.Errorf("WaitExit error 5 %v", err)
	}
	link := string(b1)

	// Remove pid from my children now its status has been
	// collected We don't need to abandon it.
	if err := clnt.Remove(procdir); err != nil {
		log.Printf("Error Remove %v in WaitExit: %v", procdir, err)
		return "", fmt.Errorf("WaitExit error 6 %v", err)
	}

	// XXX what happens if we crash here; who will collect? procd?
	clnt.removeProc(link)

	return string(b), nil
}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid string) error {
	procdir := proc.GetProcDir()
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
	procdir := proc.GetProcDir()
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.START_SEM))
	err := semStart.Up()
	if err != nil {
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
// have.  If itself is an abandoned child, then it cleans up itself;
// otherwise the parent will do it.
//
// exited() should be called *once* per proc, but procd's procclnt may
// call exited() for different procs.
func (clnt *ProcClnt) exited(procdir string, pid string, status string) error {
	// will catch some unintended misuses: a proc calling exited
	// twice or procd calling exited twice.
	if clnt.setExited(pid) == pid {
		log.Printf("%v: Exited called after exited %v\n", db.GetName(), procdir)
		return fmt.Errorf("Exited error called more than once for pid %v", pid)
	}

	// Abandon any children I may have left.
	err := clnt.abandonChildren(pid)
	if err != nil {
		return fmt.Errorf("Exited error %v", err)
	}

	pipePath := path.Join(procdir, proc.RET_STATUS)
	fd, err := clnt.Open(pipePath, np.OWRITE)
	if err != nil {
		// parent has abandoned me; clean myself up
		// log.Printf("%v: Error Open %v err %v\n", db.GetName(), fn, err)
		r := clnt.removeProc(procdir)
		if r != nil {
			return fmt.Errorf("Exited error %v", r)
		}
	} else {
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
	procdir := proc.GetProcDir()
	err := clnt.exited(procdir, pid, status)
	if err != nil {
		log.Printf("%v: exited %v err %v\n", db.GetName(), pid, err)
		os.Exit(1)
	}
}

//func (clnt *ProcClnt) ExitedErr(pid string, status string) error {
//	err := clnt.exited(pid, status)
//	if err != nil {
//		log.Printf("%v: exitederr %v err %v\n", db.GetName(), pid, err)
//	}
//	return err
//}

func (clnt *ProcClnt) ExitedProcd(pid string, status string) {
	procdir := path.Join(proc.PIDS, pid)
	err := clnt.exited(procdir, pid, status)
	if err != nil {
		// XXX maybe remove any state left of proc?
		log.Printf("%v: exited %v err %v\n", db.GetName(), pid, err)
	}
}

// ========== EVICT ==========

// Procd notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) Evict(pid string) error {
	procdir := path.Join(proc.GetProcDir(), proc.CHILDREN, pid)
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Up()
	if err != nil {
		return fmt.Errorf("Evict error %v", err)
	}
	return nil
}

// ========== GETCHILDREN ==========

// Return the pids of all children.
func (clnt *ProcClnt) GetChildren(pid string) ([]string, error) {
	procdir := path.Join(proc.PIDS, pid)
	sts, err := clnt.ReadDir(path.Join(procdir, proc.CHILDREN))
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
		r := clnt.abandonChild(path.Join(proc.PIDS, cpid))
		if r != nil && err != nil {
			err = r
		}
	}
	return err
}

// Abandon child
func (clnt *ProcClnt) abandonChild(procdir string) error {
	f := path.Join(procdir, proc.RET_STATUS)
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
		log.Printf("%v: RmDir %v err %v %v", db.GetName(), procdir, err, s)
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

// Attempt to cleanup procdir
func (clnt *ProcClnt) cleanupError(pid, procdir string, err error) error {
	// Remove symlink
	clnt.Remove(path.Join(proc.GetProcDir(), proc.CHILDREN, pid))
	clnt.removeProc(procdir)
	return err
}

func (clnt *ProcClnt) isKernelProc(pid string) bool {
	procdir := proc.GetProcDir()
	_, err := clnt.Stat(path.Join(procdir, proc.KERNEL_PROC))
	return err == nil
}
