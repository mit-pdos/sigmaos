package procbasev1

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"runtime/debug"
	"strings"

	"github.com/thanhpk/randstr"

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
	RET_STAT        = "ret-stat."
	PARENT_RET_STAT = "parent-ret-stat."
	LOCK            = "L-"
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

	clnt.makeParentRetStatFile(piddir)

	clnt.makeRetStatWaiterFile(piddir)

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

// called by parent
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

// called by parent
// Wait until a proc has exited. If the proc doesn't exist, return immediately.
func (clnt *ProcBaseClnt) WaitExit(pid string) (string, error) {
	piddir := proc.PidDir(pid)
	if _, err := clnt.Stat(piddir); err != nil {
		return "", err
	}
	// Register that we want to get a return status back
	fpath, _ := clnt.registerRetStatWaiter(piddir)

	// Wait for the process to exit
	pExitCond := sync.MakeCondNew(clnt.FsLib, piddir, EXIT_COND, nil)
	pExitCond.Wait()

	status := clnt.getRetStat(piddir, fpath)
	return status, nil
}

// called by child
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

// called by child
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

// called by child
// Mark that a process has exited.
func (clnt *ProcBaseClnt) Exited(pid string, status string) error {
	piddir := proc.PidDir(pid)

	// Write back return statuses
	del := clnt.writeBackRetStats(piddir, status)

	pExitCond := sync.MakeCondNew(clnt.FsLib, piddir, EXIT_COND, nil)
	pExitCond.Destroy()

	if del {
		if err := clnt.RmDir(piddir); err != nil {
			log.Fatalf("Error RmDir in ProcBaseClnt.writeBackRetStatNew: %v", err)
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

// called by parent
func (clnt *ProcBaseClnt) makeParentRetStatFile(piddir string) {
	pid := path.Base(piddir)
	if err := clnt.MakeFile(path.Join(piddir, PARENT_RET_STAT+pid), 0777|np.DMTMP, np.OWRITE, []byte{}); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Fatalf("Error MakeFile in ProcBaseClnt.makeParentRetStatFile: %v", err)
	}
}

type RetStatWaiters struct {
	Fpaths []string
}

// called by parent
func (clnt *ProcBaseClnt) makeRetStatWaiterFile(piddir string) {
	pid := path.Base(piddir)
	l := sync.MakeLock(clnt.FsLib, piddir, LOCK+RET_STAT+pid, true)
	l.Lock()
	defer l.Unlock()

	if err := clnt.MakeFileJson(path.Join(piddir, RET_STAT+pid), 0777, &RetStatWaiters{}); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Fatalf("Error MakeFileJson in ProcBaseClnt.makeRetStatWaiterFile: %v", err)
	}
}

// called by child
// Write back exit statuses to backwards pointers
func (clnt *ProcBaseClnt) writeBackRetStats(piddir string, status string) bool {
	pid := path.Base(piddir)
	l := sync.MakeLock(clnt.FsLib, piddir, LOCK+RET_STAT+pid, true)
	l.Lock()
	defer l.Unlock()

	clnt.SetFile(piddir+"/"+PARENT_RET_STAT+pid, []byte(status), np.NoV)
	delete := false
	if _, err := clnt.SetFile(piddir+"/"+PARENT_RET_STAT+pid, []byte(status), np.NoV); err != nil {
		delete = true
	}

	rswPath := path.Join(piddir, RET_STAT+pid)
	rsw := &RetStatWaiters{}
	if err := clnt.ReadFileJson(rswPath, rsw); err != nil {
		debug.PrintStack()
		log.Fatalf("Error ReadFileJson in ProcBaseClnt.writeBackRetStats: %v %v", rswPath, err)
	}

	for _, fpath := range rsw.Fpaths {
		fn := piddir + "/" + fpath
		if _, err := clnt.SetFile(fn, []byte(status), np.NoV); err != nil {
			debug.PrintStack()
			log.Fatalf("Error WriteFile in ProcBaseClnt.writeBackRetStats: %v %v", fn, err)
		}
	}

	if err := clnt.Remove(rswPath); err != nil {
		debug.PrintStack()
		log.Fatalf("Error Remove in ProcBaseClnt.writeBackRetStats: %v", err)
	}
	return delete
}

// called by parent
// Register backwards pointers for files in which to write return statuses
func (clnt *ProcBaseClnt) registerRetStatWaiter(piddir string) (string, error) {
	pid := path.Base(piddir)
	l := sync.MakeLock(clnt.FsLib, piddir, LOCK+RET_STAT+pid, true)
	l.Lock()
	defer l.Unlock()

	// Get & update the list of backwards pointers
	rswPath := path.Join(piddir, RET_STAT+pid)
	rsw := &RetStatWaiters{}
	if err := clnt.ReadFileJson(rswPath, rsw); err != nil {
		if err.Error() == "file not found "+RET_STAT+pid {
			return "", err
		}
		log.Fatalf("Error ReadFileJson in ProcBaseClnt.registerRetStatWaiter: %v", err)
		return "", err

	}
	// pathname for child
	fpath := path.Join(randstr.Hex(16))
	rsw.Fpaths = append(rsw.Fpaths, fpath)

	// pathname for parent
	fpath = proc.PidDir(pid) + "/" + fpath
	if err := clnt.MakeFile(fpath, 0777, np.OWRITE, []byte{}); err != nil {
		log.Fatalf("Error MakeFile %v err %v", fpath, err)
	}

	if err := clnt.WriteFileJson(rswPath, rsw); err != nil {
		log.Fatalf("Error WriteFileJson %v err %v", rswPath, err)
		return "", err
	}

	return fpath, nil
}

// called by parent
// Read & destroy a return status file
func (clnt *ProcBaseClnt) getRetStat(piddir string, fpath string) string {
	var b []byte
	var err error

	pid := path.Base(piddir)
	// If we didn't successfully register the ret stat file
	if fpath == "" {
		// Try to read the parent ret stat file
		b, _, err = clnt.GetFile(piddir + "/" + PARENT_RET_STAT + pid)

		// XXX remove file?
	} else {
		// XXX file lock?

		// Try to read the registerred ret stat file
		b, _, err = clnt.GetFile(fpath)
		if err != nil {
			log.Fatalf("Error ReadFile in ProcBaseClnt.getRetStat: %v", err)
		}

		// Remove the registerred ret stat file
		if err := clnt.Remove(fpath); err != nil {
			log.Fatalf("Error Remove in ProcBaseClnt.getRetStat: %v", err)
		}

		// XXX if parent doesn't call WaitExit(), someone should
		// Remove pid dir
		if err := clnt.RmDir(piddir); err != nil {
			log.Fatalf("Error RmDir %v in ProcBaseClnt.getRetStat: %v", piddir, err)
		}

	}

	return string(b)
}
