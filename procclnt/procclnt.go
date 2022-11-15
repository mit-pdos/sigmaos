package procclnt

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/namespace"
	np "sigmaos/ninep"
	"sigmaos/proc"
	// "sigmaos/seccomp"
	"sigmaos/semclnt"
)

type ProcClnt struct {
	sync.Mutex
	*fslib.FsLib
	pid             proc.Tpid
	isExited        proc.Tpid
	procdir         string
	procds          []string
	lastProcdUpdate time.Time
	burstOffset     int
}

func makeProcClnt(fsl *fslib.FsLib, pid proc.Tpid, procdir string) *ProcClnt {
	clnt := &ProcClnt{}
	clnt.FsLib = fsl
	clnt.pid = pid
	clnt.procdir = procdir
	return clnt
}

// ========== SPAWN ==========

// XXX Should probably eventually fold this into spawn (but for now, we may want to get the exec.Cmd struct back).
func (clnt *ProcClnt) SpawnKernelProc(p *proc.Proc, namedAddr []string, viaProcd bool) (*exec.Cmd, error) {
	if err := clnt.spawn("~ip", viaProcd, p); err != nil {
		return nil, err
	}
	if !viaProcd {
		return proc.RunKernelProc(p, namedAddr)
	}
	return nil, nil
}

// Burst-spawn a set of procs across available procds. Return a slice of procs
// which were unable to be successfully spawned, as well as corresponding
// errors.
//
// Use of burstOffset makes sure we continue rotating across invocations as
// well as within an invocation.
func (clnt *ProcClnt) SpawnBurst(ps []*proc.Proc) ([]*proc.Proc, []error) {
	failed := []*proc.Proc{}
	errs := []error{}
	for i := range ps {
		// Update the list of active procds.
		clnt.updateProcds()
		err := clnt.spawn(clnt.nextProcd(), true, ps[i])
		if err != nil {
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

// XXX Currently for benchmarking purposes... remove eventually.
func (clnt *ProcClnt) SpawnBurstParallel(ps []*proc.Proc, chunksz int) ([]*proc.Proc, []error) {
	failed := []*proc.Proc{}
	errs := []error{}
	errc := make(chan []*errTuple)
	clnt.updateProcds()
	for i := 0; i < len(ps); i += chunksz {
		go func(i int) {
			// Take a slice of procs.
			pslice := ps[i : i+chunksz]
			es := []*errTuple{}
			lastUpdate := time.Now()
			for x, p := range pslice {
				// Update the list of procds periodically, but not too often
				if time.Since(lastUpdate) >= np.Conf.Realm.RESIZE_INTERVAL {
					clnt.updateProcds()
					lastUpdate = time.Now()
				}
				// Update the list of active procds.
				_ = x
				err := clnt.spawn(clnt.nextProcd(), true, p)
				if err != nil {
					db.DPrintf(db.ALWAYS, "Error burst-spawn %v: %v", p, err)
					es = append(es, &errTuple{p, err})
				}
			}
			errc <- es
		}(i)
	}
	// Wait for spawn results.
	for i := 0; i < len(ps); i += chunksz {
		es := <-errc
		for _, e := range es {
			failed = append(failed, e.proc)
			errs = append(errs, e.err)
		}
	}
	return failed, errs
}

func (clnt *ProcClnt) Spawn(p *proc.Proc) error {
	return clnt.spawn("~ip", true, p)
}

// Spawn a proc on procdIp. If viaProcd is false, then the proc env is set up and the proc is not actually spawned on procd, since it will be started later.
func (clnt *ProcClnt) spawn(procdIp string, viaProcd bool, p *proc.Proc) error {
	if p.Ncore > 0 && p.Type != proc.T_LC {
		db.DFatalf("Spawn non-LC proc with Ncore set %v", p)
		return fmt.Errorf("Spawn non-LC proc with Ncore set %v", p)
	}
	// Set the parent dir
	p.SetParentDir(clnt.procdir)
	childProcdir := p.ProcDir

	db.DPrintf("PROCCLNT", "Spawn %v", p)
	if clnt.hasExited() != "" {
		db.DPrintf("PROCCLNT_ERR", "Spawn error called after Exited")
		db.DFatalf("Spawn error called after Exited")
	}

	if err := clnt.addChild(procdIp, p, childProcdir); err != nil {
		return err
	}

	p.SpawnTime = time.Now()
	// If this is not a privileged proc, spawn it through procd.
	if viaProcd {
		b, err := json.Marshal(p)
		if err != nil {
			db.DPrintf("PROCLNT_ERR", "Spawn marshal err %v", err)
			return clnt.cleanupError(p.Pid, childProcdir, fmt.Errorf("Spawn error %v", err))
		}
		fn := path.Join(np.PROCDREL, procdIp, np.PROCD_SPAWN_FILE)
		_, err = clnt.SetFile(fn, b, np.OWRITE, 0)
		if err != nil {
			db.DPrintf("PROCCLNT_ERR", "SetFile %v err %v", fn, err)
			return clnt.cleanupError(p.Pid, childProcdir, fmt.Errorf("Spawn error %v", err))
		}
	} else {
		// Make the proc's procdir
		err := clnt.MakeProcDir(p.Pid, p.ProcDir, p.IsPrivilegedProc())
		if err != nil {
			db.DPrintf("PROCCLNT_ERR", "Err SpawnKernelProc MakeProcDir: %v", err)
		}
		// Create a semaphore to indicate a proc has started if this is a kernel
		// proc. Otherwise, procd will create the semaphore.
		childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, p.Pid))
		semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
		semStart.Init(0)
	}

	return nil
}

// Update the list of active procds.
func (clnt *ProcClnt) updateProcds() {
	clnt.Lock()
	defer clnt.Unlock()

	// If we updated the list of active procds recently, return immediately. The
	// list will change at most as quickly as the realm resizes.
	if time.Since(clnt.lastProcdUpdate) < np.Conf.Realm.RESIZE_INTERVAL {
		return
	}
	clnt.lastProcdUpdate = time.Now()
	// Read the procd union dir.
	procds, _, err := clnt.ReadDir(np.PROCDREL)
	if err != nil {
		db.DFatalf("Error ReadDir procd: %v", err)
	}
	// Alloc enough space for the list of procds, excluding the ws queue.
	clnt.procds = make([]string, 0, len(procds)-1)
	for _, procd := range procds {
		if procd.Name != path.Base(path.Dir(np.PROCD_WS)) {
			clnt.procds = append(clnt.procds, procd.Name)
		}
	}
}

// Get the next procd to burst on.
func (clnt *ProcClnt) nextProcd() string {
	clnt.Lock()
	defer clnt.Unlock()

	if len(clnt.procds) == 0 {
		db.DFatalf("Error: no procds to spawn on")
	}

	clnt.burstOffset++
	return clnt.procds[clnt.burstOffset%len(clnt.procds)]
}

// XXX For short-term benchmarking only.
func (clnt *ProcClnt) nextProcdUnsafe(i int) string {
	if len(clnt.procds) == 0 {
		db.DFatalf("Error: no procds to spawn on")
	}

	return clnt.procds[i%len(clnt.procds)]
}

// ========== WAIT ==========

func (clnt *ProcClnt) waitStart(pid proc.Tpid) error {
	childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.PROCFILE_LINK))
	if err != nil {
		db.DPrintf("PROCCLNT_ERR", "Can't get procip file: %v", err)
		return err
	}
	procfileLink := string(b)
	// Kernel procs will have empty proc file links.
	if procfileLink != "" {
		db.DPrintf("PROCCLNT", "%v set remove watch: %v", pid, procfileLink)
		done := make(chan bool)
		err := clnt.SetRemoveWatch(procfileLink, func(string, error) {
			done <- true
		})
		if err != nil {
			db.DPrintf("PROCCLNT_ERR", "Error waitStart SetRemoveWatch %v", err)
			if np.IsErrUnreachable(err) {
				return err
			}
		} else {
			<-done
		}
	}
	db.DPrintf("PROCCLNT", "WaitStart %v %v", pid, childDir)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
	return semStart.Down()
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid proc.Tpid) error {
	err := clnt.waitStart(pid)
	if err != nil {
		db.DPrintf("PROCCLNT_ERR", "WaitStart %v %v", pid, err)
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
		db.DPrintf("PROCCLNT", "waitStart err %v", err)
	}

	db.DPrintf("PROCCLNT", "WaitExit %v", pid)

	// Make sure the child proc has exited.
	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(proc.GetChildProcDir(clnt.procdir, pid), proc.EXIT_SEM))
	if err := semExit.Down(); err != nil {
		db.DPrintf("PROCCLNT_ERR", "Error WaitExit semExit.Down: %v", err)
		return nil, fmt.Errorf("Error semExit.Down: %v", err)
	}

	defer clnt.removeChild(pid)

	childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.EXIT_STATUS))
	if err != nil {
		db.DPrintf("PROCCLNT_ERR", "Missing return status, procd must have crashed: %v, %v", pid, err)
		return nil, fmt.Errorf("Missing return status, procd must have crashed: %v", err)
	}

	status := &proc.Status{}
	if err := json.Unmarshal(b, status); err != nil {
		db.DPrintf("PROCCLNT_ERR", "waitexit unmarshal err %v", err)
		return nil, err
	}

	return status, nil
}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid proc.Tpid) error {
	db.DPrintf("PROCCLNT", "WaitEvict %v", pid)
	procdir := proc.PROCDIR
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Down()
	if err != nil {
		return fmt.Errorf("WaitEvict error %v", err)
	}
	db.DPrintf("PROCCLNT", "WaitEvict evicted %v", pid)
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started() error {
	db.DPrintf("PROCCLNT", "Started %v", clnt.pid)

	// Link self into parent dir
	if err := clnt.linkSelfIntoParentDir(); err != nil {
		db.DPrintf("PROCCLNT", "linkSelfIntoParentDir %v err %v", clnt.pid, err)
		return err
	}

	// Mark self as started
	semPath := path.Join(proc.PARENTDIR, proc.START_SEM)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, semPath)
	err := semStart.Up()
	if err != nil {
		db.DPrintf("PROCCLNT_ERR", "Started error %v %v", semPath, err)
	}
	// File may not be found if parent exited first or isn't reachable
	if err != nil && !np.IsErrUnavailable(err) {
		return fmt.Errorf("Started error %v", err)
	}
	// Only isolate non-kernel procs
	if !proc.GetIsPrivilegedProc() {
		// Isolate the process namespace
		newRoot := proc.GetNewRoot()
		if err := namespace.Isolate(newRoot); err != nil {
			db.DPrintf("PROCCLNT_ERR", "Error Isolate in clnt.Started: %v", err)
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
	db.DPrintf("PROCCLNT", "exited %v parent %v pid %v status %v", procdir, parentdir, pid, status)

	// will catch some unintended misuses: a proc calling exited
	// twice or procd calling exited twice.
	if clnt.setExited(pid) == pid {
		db.DFatalf("Exited called after exited %v", procdir)
	}

	b, err := json.Marshal(status)
	if err != nil {
		db.DPrintf("PROCCLNT_ERR", "exited marshal err %v", err)
		return err
	}
	// May return an error if parent already exited.
	fn := path.Join(parentdir, proc.EXIT_STATUS)
	if _, err := clnt.PutFile(fn, 0777, np.OWRITE, b); err != nil {
		db.DPrintf("PROCCLNT_ERR", "exited error (parent already exited) MakeFile %v err %v", fn, err)
	}

	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EXIT_SEM))
	if err := semExit.Up(); err != nil {
		db.DPrintf("PROCCLNT_ERR", "exited semExit up error: %v, %v, %v", procdir, pid, err)
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
	err := clnt.exited(clnt.procdir, proc.PARENTDIR, proc.GetPid(), status)
	if err != nil {
		db.DFatalf("exited %v err %v", proc.GetPid(), err)
	}
	clnt.FsLib.Exit()
}

func (clnt *ProcClnt) ExitedProcd(pid proc.Tpid, procdir string, parentdir string, status *proc.Status) {
	db.DPrintf("PROCCLNT", "exited %v parent %v pid %v status %v", procdir, parentdir, pid, status)
	err := clnt.exited(procdir, parentdir, pid, status)
	if err != nil {
		// XXX maybe remove any state left of proc?
		db.DPrintf("PROCCLNT_ERR", "exited %v err %v", pid, err)
	}
	// If proc ran, but crashed before calling Started, the parent may block indefinitely. Stop this from happening by calling semStart.Up()
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(parentdir, proc.START_SEM))
	semStart.Up()
}

// ========== EVICT ==========

// Notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) evict(procdir string) error {
	db.DPrintf("PROCCLNT", "evict %v", procdir)
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Up()
	if err != nil {
		return fmt.Errorf("Evict error %v", err)
	}
	return nil
}

// Called by parent.
func (clnt *ProcClnt) Evict(pid proc.Tpid) error {
	procdir := proc.GetChildProcDir(clnt.procdir, pid)
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

func (clnt *ProcClnt) hasExited() proc.Tpid {
	clnt.Lock()
	defer clnt.Unlock()
	return clnt.isExited
}

func (clnt *ProcClnt) setExited(pid proc.Tpid) proc.Tpid {
	clnt.Lock()
	defer clnt.Unlock()
	r := clnt.isExited
	clnt.isExited = pid
	return r
}
