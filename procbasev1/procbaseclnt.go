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
}

func MakeProcBaseClnt(fsl *fslib.FsLib, pid string) *ProcBaseClnt {
	clnt := &ProcBaseClnt{}
	clnt.runq = sync.MakeFilePriorityBag(fsl, RUNQ)
	clnt.FsLib = fsl

	if pid != "" {
		if err := fsl.MountTree(fslib.Named(), pid, pid); err != nil {
			log.Fatalf("Error mounting %v err %v\n", pid, err)
		}
	}
	return clnt
}

// called by parent
func childDir(pid string) string {
	return "name/" + pid + "/"
}

// ========== SPAWN ==========

func (clnt *ProcBaseClnt) Spawn(gp proc.GenericProc) error {
	log.Printf("%v: V1\n", db.GetName())
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

	dir := childDir(p.Pid)
	if err := clnt.Mkdir(dir, 0777); err != nil {
		log.Printf("DIR FAILED %v %v\n", dir, err)
		return err
	}

	pStartCond := sync.MakeCondNew(clnt.FsLib, dir, START_COND, nil)
	pStartCond.Init()

	pExitCond := sync.MakeCondNew(clnt.FsLib, dir, EXIT_COND, nil)
	pExitCond.Init()

	pEvictCond := sync.MakeCondNew(clnt.FsLib, dir, EVICT_COND, nil)
	pEvictCond.Init()

	clnt.makeParentRetStatFile(p.Pid)

	clnt.makeRetStatWaiterFile(p.Pid)

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
	if _, err := clnt.Stat(childDir(pid)); err != nil {
		return err
	}
	pStartCond := sync.MakeCondNew(clnt.FsLib, childDir(pid), START_COND, nil)
	pStartCond.Wait()
	return nil
}

// called by parent
// Wait until a proc has exited. If the proc doesn't exist, return immediately.
func (clnt *ProcBaseClnt) WaitExit(pid string) (string, error) {
	if _, err := clnt.Stat(childDir(pid)); err != nil {
		return "", err
	}
	// Register that we want to get a return status back
	fpath, _ := clnt.registerRetStatWaiter(pid)

	// Wait for the process to exit
	pExitCond := sync.MakeCond(clnt.FsLib, childDir(pid)+EXIT_COND, nil)
	pExitCond.Wait()

	status := clnt.getRetStat(pid, fpath)

	return status, nil
}

// called by parent
// Wait for a proc's eviction notice. If the proc doesn't exist, return immediately.
func (clnt *ProcBaseClnt) WaitEvict(pid string) error {
	pEvictCond := sync.MakeCondNew(clnt.FsLib, childDir(pid), EVICT_COND, nil)
	pEvictCond.Wait()
	return nil
}

// ========== STARTED ==========

// called by child
// Mark that a process has started.
func (clnt *ProcBaseClnt) Started(pid string) error {
	dir := pid + "/"
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
	dir := pid + "/"

	// Write back return statuses
	clnt.writeBackRetStats(pid, status)

	pExitCond := sync.MakeCondNew(clnt.FsLib, dir, EXIT_COND, nil)
	pExitCond.Destroy()
	return nil
}

// ========== EVICT ==========

// Notify a process that it will be evicted.
func (clnt *ProcBaseClnt) Evict(pid string) error {

	pEvictCond := sync.MakeCondNew(clnt.FsLib, childDir(pid), EVICT_COND, nil)
	pEvictCond.Destroy()
	return nil
}

// ========== Helpers ==========

// called by parent
func (clnt *ProcBaseClnt) makeParentRetStatFile(pid string) {
	dir := childDir(pid)
	if err := clnt.MakeFile(path.Join(dir, PARENT_RET_STAT+pid), 0777|np.DMTMP, np.OWRITE, []byte{}); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Fatalf("Error MakeFile in ProcBaseClnt.makeParentRetStatFile: %v", err)
	}
}

type RetStatWaiters struct {
	Fpaths []string
}

// called by parent
func (clnt *ProcBaseClnt) makeRetStatWaiterFile(pid string) {
	dir := childDir(pid)
	l := sync.MakeLock(clnt.FsLib, dir, LOCK+RET_STAT+pid, true)
	l.Lock()
	defer l.Unlock()

	if err := clnt.MakeFileJson(path.Join(dir, RET_STAT+pid), 0777, &RetStatWaiters{}); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Fatalf("Error MakeFileJson in ProcBaseClnt.makeRetStatWaiterFileNew: %v", err)
	}
}

// called by child
// Write back exit statuses to backwards pointers
func (clnt *ProcBaseClnt) writeBackRetStats(pid string, status string) {
	l := sync.MakeLock(clnt.FsLib, pid, LOCK+RET_STAT+pid, true)
	l.Lock()
	defer l.Unlock()

	clnt.SetFile(pid+"/"+PARENT_RET_STAT+pid, []byte(status), np.NoV)

	rswPath := path.Join(pid, RET_STAT+pid)
	rsw := &RetStatWaiters{}
	if err := clnt.ReadFileJson(rswPath, rsw); err != nil {
		debug.PrintStack()
		log.Fatalf("Error ReadFileJson in ProcBaseClnt.writeBackRetStatsNew: %v %v", rswPath, err)
	}

	for _, fpath := range rsw.Fpaths {
		if _, err := clnt.SetFile(fpath, []byte(status), np.NoV); err != nil {
			debug.PrintStack()
			log.Fatalf("Error WriteFile in ProcBaseClnt.writeBackRetStatsNew: %v", err)
		}
	}

	if err := clnt.Remove(rswPath); err != nil {
		debug.PrintStack()
		log.Fatalf("Error Remove in ProcBaseClnt.writeBackRetStatsNew: %v", err)
	}
}

// called by parent
// Register backwards pointers for files in which to write return statuses
func (clnt *ProcBaseClnt) registerRetStatWaiter(pid string) (string, error) {
	dir := childDir(pid)
	l := sync.MakeLock(clnt.FsLib, dir, LOCK+RET_STAT+pid, true)
	l.Lock()
	defer l.Unlock()

	// Get & update the list of backwards pointers
	rswPath := path.Join(dir, RET_STAT+pid)
	rsw := &RetStatWaiters{}
	if err := clnt.ReadFileJson(rswPath, rsw); err != nil {
		if err.Error() == "file not found "+RET_STAT+pid {
			return "", err
		}
		log.Fatalf("Error ReadFileJson in ProcBaseClnt.registerRetStatWaiterNew: %v", err)
		return "", err

	}
	// pathname for child
	fpath := path.Join(pid, randstr.Hex(16))
	rsw.Fpaths = append(rsw.Fpaths, fpath)

	// pathname for parent
	fpath = "name/" + fpath
	if err := clnt.MakeFile(fpath, 0777, np.OWRITE, []byte{}); err != nil {
		log.Fatalf("Error MakeFile in ProcBaseClnt.registerRetStatWaiterNew: %v", err)
	}

	if err := clnt.WriteFileJson(rswPath, rsw); err != nil {
		log.Fatalf("Error WriteFileJson in ProcBaseClnt.registerRetStatWaiterNew: %v", err)
		return "", err
	}

	return fpath, nil
}

// called by parent
// Read & destroy a return status file
func (clnt *ProcBaseClnt) getRetStat(pid string, fpath string) string {
	var b []byte
	var err error

	log.Printf("getRetStatNew: %v\n", fpath)
	// If we didn't successfully register the ret stat file
	if fpath == "" {
		// Try to read the parent ret stat file
		b, _, err = clnt.GetFile(path.Join(childDir(pid), PARENT_RET_STAT+pid))

		// XXX remove file?
	} else {
		// Try to read the registerred ret stat file
		b, _, err = clnt.GetFile(fpath)
		if err != nil {
			log.Fatalf("Error ReadFile in ProcBaseClnt.getRetStatNew: %v", err)
		}

		// Remove the registerred ret stat file
		if err := clnt.Remove(fpath); err != nil {
			log.Fatalf("Error Remove in ProcBaseClnt.getRetStatNew: %v", err)
		}
	}

	return string(b)
}
