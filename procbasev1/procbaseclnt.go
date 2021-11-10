package procbasev1

import (
	"encoding/json"
	"log"
	"os"
	"path"
	// "runtime/debug"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/seccomp"
	"ulambda/sync"
)

type Twait uint32

const (
	START Twait = 0
	EXIT  Twait = 1
)

const (
	RUNQ = "name/runq"
)

const (
	START_COND      = "start-cond."
	EVICT_COND      = "evict-cond."
	EXIT_COND       = "exit-cond."
	PARENT_RET_STAT = "parent-ret-stat."
	LOCK            = "L-"
	CHILD           = "childs"
	PIDS            = "pids"
)

const (
	RUNQLC_PRIORITY = "0"
	RUNQ_PRIORITY   = "1"
)

type ProcBaseClnt struct {
	runq *sync.FilePriorityBag
	*fslib.FsLib
	pid    string
	piddir string
}

func MakeProcBaseClnt(fsl *fslib.FsLib, piddir, pid string) *ProcBaseClnt {
	clnt := &ProcBaseClnt{}
	clnt.runq = sync.MakeFilePriorityBag(fsl, RUNQ)
	clnt.FsLib = fsl
	clnt.pid = pid
	clnt.piddir = piddir
	return clnt
}

// ========== SPAWN ==========

// XXX cleanup on failure
func (clnt *ProcBaseClnt) Spawn(gp proc.GenericProc) error {
	p := gp.GetProc()
	// Select which queue to put the job in
	var procPriority string
	switch p.Type {
	case proc.T_DEF:
		procPriority = RUNQ_PRIORITY
	case proc.T_LC:
		procPriority = RUNQLC_PRIORITY
	case proc.T_BE:
		procPriority = RUNQ_PRIORITY
	default:
		log.Fatalf("Error in ProcBaseClnt.Spawn: Unknown proc type %v", p.Type)
	}

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
	pStartCond := sync.MakeCondNew(clnt.FsLib, piddir, START_COND, nil)
	pStartCond.Init()

	pExitCond := sync.MakeCondNew(clnt.FsLib, piddir, EXIT_COND, nil)
	pExitCond.Init()

	pEvictCond := sync.MakeCondNew(clnt.FsLib, piddir, EVICT_COND, nil)
	pEvictCond.Init()

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
		pStartCond.Destroy()
		pExitCond.Destroy()
		pEvictCond.Destroy()
		log.Fatalf("Error marshal: %v", err)
		return err
	}

	err = clnt.runq.Put(procPriority, p.Pid, b)
	if err != nil {
		log.Printf("Error Put in ProcBaseClnt.Spawn: %v", err)
		return err
	}

	return nil
}

// ========== WAIT ==========

// Wait until a proc has started. If the proc doesn't exist, return immediately.
func (clnt *ProcBaseClnt) WaitStart(pid string) error {
	piddir := proc.PidDir(pid)
	if _, err := clnt.Stat(piddir); err != nil {
		return err
	}
	pStartCond := sync.MakeCondNew(clnt.FsLib, piddir, START_COND, nil)
	pStartCond.Wait()
	return nil
}

// Wait until a proc has exited. If the proc doesn't exist, return immediately.
// Should be called only by parent
func (clnt *ProcBaseClnt) WaitExit(pid string) (string, error) {
	piddir := proc.PidDir(pid)

	if _, err := clnt.Stat(piddir); err != nil {
		log.Printf("waitexit: child doesn't exist!\n")
		return "", err
	}

	// Wait for the process to exit
	pExitCond := sync.MakeCondNew(clnt.FsLib, piddir, EXIT_COND, nil)
	pExitCond.Wait()

	status := clnt.getRetStat(piddir)

	// Remove pid from my children
	f := PIDS + "/" + proc.GetPid() + "/" + CHILD + "/" + path.Base(pid)
	if err := clnt.Remove(f); err != nil {
		log.Fatalf("Error Remove %v in WaitExit: %v", f, err)
	}

	// Remove proc
	if err := clnt.RmDir(piddir); err != nil {
		log.Fatalf("Error RmDir %v in WaitExit: %v", piddir, err)
	}

	return status, nil

}

// Wait for a proc's eviction notice. If the proc doesn't exist, return immediately.
func (clnt *ProcBaseClnt) WaitEvict(pid string) error {
	piddir := proc.PidDir(pid)
	if _, err := clnt.Stat(piddir); err != nil {
		return err
	}
	pEvictCond := sync.MakeCondNew(clnt.FsLib, piddir, EVICT_COND, nil)
	pEvictCond.Wait()
	return nil
}

// ========== STARTED ==========

// Mark that a process has started.
func (clnt *ProcBaseClnt) Started(pid string) error {
	dir := proc.PidDir(pid)
	if _, err := clnt.Stat(dir); err != nil {
		return err
	}
	pStartCond := sync.MakeCondNew(clnt.FsLib, dir, START_COND, nil)
	pStartCond.Destroy()
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

// Mark that a process has exited.  XXX are there races between parent
// exit/abandon and child exited, and between parent calling waitexit
// and child exited()?  For example, child write ret status file
// successfully, parent exits w.o. calling waitexit() for child and
// removes ret status file, and child should remove itself but it
// won't.  Probably need lock on proc dir.
func (clnt *ProcBaseClnt) Exited(pid string, status string) error {
	piddir := proc.PidDir(pid)

	// Write back return statuses
	ok := clnt.writeBackRetStats(piddir, status)

	// Abandon any children I may have left
	clnt.abandonChildren(piddir)
	if ok {
		// wakekup parent if it called WaitExit()
		pExitCond := sync.MakeCondNew(clnt.FsLib, piddir, EXIT_COND, nil)
		pExitCond.Destroy()
	} else {
		// parent has abandoned me; clean myself up
		if err := clnt.RmDir(piddir); err != nil {
			log.Fatalf("Error RmDir %v in WaitExit: %v", piddir, err)
		}
	}
	return nil
}

// ========== EVICT ==========

// Notify a process that it will be evicted.
func (clnt *ProcBaseClnt) Evict(pid string) error {
	piddir := proc.PidDir(pid)
	if _, err := clnt.Stat(piddir); err != nil {
		return err
	}
	pEvictCond := sync.MakeCondNew(clnt.FsLib, piddir, EVICT_COND, nil)
	pEvictCond.Destroy()
	return nil
}

// ========== Helpers ==========

func (clnt *ProcBaseClnt) makeParentRetStatFile(piddir string) {
	if err := clnt.MakeFile(path.Join(piddir, PARENT_RET_STAT), 0777, np.OWRITE, []byte{}); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Fatalf("Error MakeFile in ProcBaseClnt.makeParentRetStatFile: %v", err)
	}
}

// Read return status file
func (clnt *ProcBaseClnt) getRetStat(piddir string) string {
	var b []byte
	var err error

	b, _, err = clnt.GetFile(piddir + "/" + PARENT_RET_STAT)
	if err != nil {
		log.Fatalf("Error ReadFile in ProcBaseClnt.getRetStat: %v", err)
	}
	return string(b)
}

// Write back exit status
func (clnt *ProcBaseClnt) writeBackRetStats(piddir string, status string) bool {
	fn := piddir + "/" + PARENT_RET_STAT
	if _, err := clnt.SetFile(fn, []byte(status), np.NoV); err != nil {
		// parent has abandoned me
		log.Printf("%v: writeBackRetStats: status file %v err %v\n", db.GetName(), fn, err)
		return false
	}
	return true
}

// Remove status from children to indicate we don't care
func (clnt *ProcBaseClnt) abandonChildren(piddir string) {
	cpids := piddir + "/" + CHILD
	sts, err := clnt.ReadDir(cpids)
	if err != nil {
		log.Fatalf("abandonChildren %v err : %v", cpids, err)
	}
	for _, st := range sts {
		f := PIDS + "/" + st.Name + "/" + PARENT_RET_STAT
		if err := clnt.Remove(f); err != nil {
			log.Fatalf("%v: abandonChildren rmfile child %v err %v\n", db.GetName(), f, err)
		}
	}
}
