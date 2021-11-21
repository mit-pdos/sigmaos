package procclnt

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/seccomp"
	usync "ulambda/sync"
)

const (
	// name for dir where procs live. May not refer to name/pids
	// because proc.PidDir may change it.  A proc refers to itself
	// using "pids/<pid>", where pid is the proc's PID.
	PIDS = "pids"

	// Files/directories in "pids/<pid>":
	START_WAIT = "start-cond."
	EVICT_WAIT = "evict-cond."
	EXIT_WAIT  = "exit-cond."
	RET_STATUS = "ret-status."
	LOCK       = "pid-lock."
	CHILD      = "childs" // directory with children's pids
)

type ProcClnt struct {
	mu sync.Mutex
	*fslib.FsLib
	pid    string
	piddir string
	exited string
}

func makeProcClnt(fsl *fslib.FsLib, piddir, pid string) *ProcClnt {
	clnt := &ProcClnt{}
	clnt.FsLib = fsl
	clnt.pid = pid
	clnt.piddir = piddir
	return clnt
}

// ========== SPAWN ==========

// XXX cleanup on failure
func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	if clnt.hasExited() == p.Pid {
		return fmt.Errorf("Spawn: called after Exited")
	}

	// Select which queue to put the job in
	piddir := proc.PidDir(p.Pid)
	if err := clnt.Mkdir(piddir, 0777); err != nil {
		log.Fatalf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), piddir, err)
		return err
	}
	if clnt.piddir != p.PidDir {
		log.Printf("%v: spawn clnt %v make piddir %v\n", db.GetName(), clnt.piddir, p.PidDir)
		if err := clnt.Mkdir(p.PidDir, 0777); err != nil {
			log.Fatalf("%v: Spawn new piddir %v err %v\n", db.GetName(), p.PidDir, err)
			return err
		}
		piddir = p.PidDir + "/" + p.Pid
		if err := clnt.Mkdir(piddir, 0777); err != nil {
			log.Fatalf("%v: Spawn mkdir pid %v err %v\n", db.GetName(), piddir, err)
			return err
		}
	}

	pStartWait := usync.MakeWait(clnt.FsLib, piddir, START_WAIT)
	pStartWait.Init()

	pExitWait := usync.MakeWait(clnt.FsLib, piddir, EXIT_WAIT)
	pExitWait.Init()

	pEvictWait := usync.MakeWait(clnt.FsLib, piddir, EVICT_WAIT)
	pEvictWait.Init()

	d := piddir + "/" + CHILD
	if err := clnt.Mkdir(d, 0777); err != nil {
		log.Fatalf("%v: Spawn mkdir childs %v err %v\n", db.GetName(), d, err)
		return err
	}

	clnt.makeParentRetStatFile(piddir)

	// Add pid to my children
	f := PIDS + "/" + proc.GetPid() + "/" + CHILD + "/" + p.Pid
	if err := clnt.MakeFile(f, 0777, np.OWRITE, []byte{}); err != nil {
		log.Fatalf("%v: Spawn mkfile child %v err %v\n", db.GetName(), f, err)
		return err
	}

	b, err := json.Marshal(p)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		pStartWait.Destroy()
		pExitWait.Destroy()
		pEvictWait.Destroy()
		log.Fatalf("Error marshal: %v", err)
		return err
	}

	err = clnt.WriteFile(path.Join(named.PROCDDIR+"/~ip", named.PROC_CTL_FILE), b)
	if err != nil {
		log.Printf("Error WriteFile in ProcClnt.Spawn: %v", err)
		return err
	}

	return nil
}

// ========== WAIT ==========

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid string) error {
	piddir := proc.PidDir(pid)
	if _, err := clnt.Stat(piddir); err != nil {
		return err
	}
	pStartWait := usync.MakeWait(clnt.FsLib, piddir, START_WAIT)
	pStartWait.Wait()
	return nil
}

// Parent calls WaitExited() to wait until child proc has exited. If
// the proc doesn't exist, return immediately.  After collecting
// return status, parent cleans up the child and parent removes the
// child from its list of children.
func (clnt *ProcClnt) WaitExit(pid string) (string, error) {
	piddir := proc.PidDir(pid)

	if _, err := clnt.Stat(piddir); err != nil {
		return "", err
	}

	// Wait for the process to exit
	pExitWait := usync.MakeWait(clnt.FsLib, piddir, EXIT_WAIT)
	pExitWait.Wait()

	// Remove pid from my children
	f := PIDS + "/" + proc.GetPid() + "/" + CHILD + "/" + path.Base(pid)
	if err := clnt.Remove(f); err != nil {
		log.Fatalf("Error Remove %v in WaitExit: %v", f, err)
	}

	// Collect status and remove child
	status := clnt.collectChild(piddir)

	return status, nil

}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid string) error {
	piddir := proc.PidDir(pid)
	if _, err := clnt.Stat(piddir); err != nil {
		return err
	}
	pEvictWait := usync.MakeWait(clnt.FsLib, piddir, EVICT_WAIT)
	pEvictWait.Wait()
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started(pid string) error {
	dir := proc.PidDir(pid)
	if _, err := clnt.Stat(dir); err != nil {
		return err
	}
	pStartWait := usync.MakeWait(clnt.FsLib, dir, START_WAIT)
	pStartWait.Destroy()
	// Isolate the process namespace
	newRoot := os.Getenv("NEWROOT")
	if err := namespace.Isolate(newRoot); err != nil {
		log.Fatalf("Error Isolate in clnt.Started: %v", err)
	}
	// Load a seccomp filter.
	seccomp.LoadFilter()
	return nil
}

// ========== EXITED ==========

// Proc pid mark itself as exited. Typically Exited() is called by
// proc pid, but if the proc crashes, procd calls Exited() on behalf
// of the failed proc. The exited proc abandons any chidren it may
// have.  If itself is an abandoned child, then it cleans up itself;
// otherwise the parent will do it.
//
// Exited() should be called *once* per proc, but procd's procclnt may
// call Exited() for different procs.
func (clnt *ProcClnt) Exited(pid string, status string) error {
	piddir := proc.PidDir(pid)

	if clnt.setExited(pid) == pid {
		log.Printf("%v: Exited called after exited\n", db.GetName())
		return fmt.Errorf("Exited: called more than once for pid %v", pid)
	}

	// Abandon any children I may have left.  Do it befoe
	// writeBackRetStats, since after that call piddir may not
	// exist.
	clnt.abandonChildren(piddir)

	// Write back return statuses; if successful parent may
	// collect me now because it exited too without calling
	// WaitExit().
	ok := clnt.writeBackRetStats(piddir, status)

	if ok {
		// wakekup parent in case it called WaitExit()
		pExitWait := usync.MakeWait(clnt.FsLib, piddir, EXIT_WAIT)
		pExitWait.Destroy()
	} else {
		// parent has abandoned me; clean myself up
		clnt.destroyProc(piddir)
	}
	return nil
}

// ========== EVICT ==========

// Procd notifies a proc that it will be evicted using Evict.  XXX
// race between procd's call to evict() and parent/child: between
// procd stat-ing and Destroy() parent/child may have removed the
// piddir.
func (clnt *ProcClnt) Evict(pid string) error {
	piddir := proc.PidDir(pid)
	if _, err := clnt.Stat(piddir); err != nil {
		return err
	}
	pEvictWait := usync.MakeWait(clnt.FsLib, piddir, EVICT_WAIT)
	pEvictWait.Destroy()
	return nil
}

// ========== Helpers ==========

func (clnt *ProcClnt) makeParentRetStatFile(piddir string) {
	if err := clnt.MakeFile(path.Join(piddir, RET_STATUS), 0777, np.OWRITE, []byte{}); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Fatalf("Error MakeFile in ProcClnt.makeParentRetStatFile: %v", err)
	}
}

// Read return status file
func (clnt *ProcClnt) getRetStat(fn string) string {
	var b []byte
	var err error

	b, _, err = clnt.GetFile(fn)
	if err != nil {
		log.Fatalf("%v: GetFile %v err %v", db.GetName(), fn, err)
	}
	s := string(b)
	return s
}

// Write back exit status
func (clnt *ProcClnt) writeBackRetStats(piddir string, status string) bool {
	fn := piddir + "/" + RET_STATUS
	if _, err := clnt.SetFile(fn, []byte(status), np.NoV); err != nil {
		// if failed to write, parent has abandoned me for sure
		return false
	}
	f := piddir + "/" + RET_STATUS
	fc := piddir + "/" + RET_STATUS + "-c"
	err := clnt.Rename(f, fc)
	if err != nil { // parent abandoned me
		return false
	}
	return true
}

// Remove status from children to indicate we don't care
func (clnt *ProcClnt) abandonChildren(piddir string) {
	cpids := piddir + "/" + CHILD
	sts, err := clnt.ReadDir(cpids)
	if err != nil {
		log.Fatalf("abandonChildren %v err : %v", cpids, err)
	}
	for _, st := range sts {
		clnt.abandonChild(PIDS + "/" + st.Name)
	}
}

// Abandon child or collect it, depending on RET_STATUS
func (clnt *ProcClnt) abandonChild(piddir string) {
	f := piddir + "/" + RET_STATUS
	fp := piddir + "/" + RET_STATUS + "-p"
	err := clnt.Rename(f, fp)
	if err != nil { // abandoning child failed, collect it
		clnt.collectChild(piddir)
	}
}

// Clean up proc
func (clnt *ProcClnt) destroyProc(piddir string) {
	if err := clnt.RmDir(piddir); err != nil {
		s, _ := clnt.SprintfDir(piddir)
		log.Fatalf("%v: RmDir %v err %v %v", db.GetName(), piddir, err, s)
	}
}

func (clnt *ProcClnt) collectChild(piddir string) string {
	status := clnt.getRetStat(piddir + "/" + RET_STATUS + "-c")
	clnt.destroyProc(piddir)
	return status
}

func (clnt *ProcClnt) hasExited() string {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()
	return clnt.exited
}

func (clnt *ProcClnt) setExited(pid string) string {
	clnt.mu.Lock()
	defer clnt.mu.Unlock()
	r := clnt.exited
	clnt.exited = pid
	return r
}
