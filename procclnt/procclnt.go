package procclnt

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"runtime/debug"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kproc"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	schedd "sigmaos/schedd/proto"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Thow uint32

const (
	HPROCD  Thow = iota + 1 // spawned as a sigmos proc
	HLINUX                  // spawned as a linux process
	HDOCKER                 // spawned as a container
)

type ProcClnt struct {
	sync.Mutex
	*fslib.FsLib
	pid             proc.Tpid
	isExited        proc.Tpid
	procdir         string
	schedds         map[string]*protdevclnt.ProtDevClnt
	scheddIps       []string
	lastProcdUpdate time.Time
	burstOffset     int
}

func makeProcClnt(fsl *fslib.FsLib, pid proc.Tpid, procdir string) *ProcClnt {
	clnt := &ProcClnt{}
	clnt.FsLib = fsl
	clnt.pid = pid
	clnt.procdir = procdir
	clnt.schedds = make(map[string]*protdevclnt.ProtDevClnt)
	clnt.scheddIps = make([]string, 0)
	return clnt
}

// ========== SPAWN ==========

// Create the named state the proc (and its parent) expects.
func (clnt *ProcClnt) MkProc(p *proc.Proc, namedAddr []string, realm string, how Thow) error {
	// Always spawn kernel procs on the local kernel.
	scheddIp := "~local"
	return clnt.spawn(scheddIp, how, p, clnt.getScheddClnt(scheddIp))
}

func (clnt *ProcClnt) SpawnKernelProc(p *proc.Proc, namedAddr []string, realm string, how Thow) (*exec.Cmd, error) {
	if err := clnt.MkProc(p, namedAddr, realm, how); err != nil {
		return nil, err
	}
	if how == HLINUX {
		// If this proc wasn't intended to be spawned through procd, run it
		// as a local Linux process
		return kproc.RunKernelProc(p, namedAddr, realm)
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
		clnt.updateSchedds()
		scheddIp := clnt.nextSchedd()
		err := clnt.spawn(scheddIp, HPROCD, ps[i], clnt.getScheddClnt(scheddIp))
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
	clnt.updateSchedds()
	for i := 0; i < len(ps); i += chunksz {
		go func(i int) {
			// Take a slice of procs.
			pslice := ps[i : i+chunksz]
			es := []*errTuple{}
			lastUpdate := time.Now()
			for _, p := range pslice {
				// Update the list of procds periodically, but not too often
				if time.Since(lastUpdate) >= sp.Conf.Realm.RESIZE_INTERVAL {
					clnt.updateSchedds()
					lastUpdate = time.Now()
				}
				scheddIp := clnt.nextSchedd()
				// Update the list of active procds.
				err := clnt.spawn(scheddIp, HPROCD, p, clnt.getScheddClnt(scheddIp))
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
	return clnt.spawn("~local", HPROCD, p, clnt.getScheddClnt("~local"))
}

func (clnt *ProcClnt) extendBaseEnv(p *proc.Proc) {
	spath := proc.GetSigmaPath()
	if spath == "" {
		if p.Privileged {
			spath = path.Join(sp.S3, "~local", clnt.Realm().String(), "/bin/kernel")
		} else {
			spath = path.Join(sp.S3, "~local", clnt.Realm().String(), "/bin/user")
		}
	}
	p.AppendEnv(proc.SIGMAPATH, spath)
	p.AppendEnv(proc.SIGMAREALM, clnt.Realm().String())
	p.AppendEnv(proc.SIGMANAMED, clnt.NamedAddr().String())
}

// Spawn a proc on scheddIp. If viaProcd is false, then the proc env is set up
// and the proc is not actually spawned on procd, since it will be started
// later.
func (clnt *ProcClnt) spawn(scheddIp string, how Thow, p *proc.Proc, pdc *protdevclnt.ProtDevClnt) error {
	if p.GetNcore() > 0 && p.GetType() != proc.T_LC {
		db.DFatalf("Spawn non-LC proc with Ncore set %v", p)
		return fmt.Errorf("Spawn non-LC proc with Ncore set %v", p)
	}

	clnt.extendBaseEnv(p)

	// Set the realm id.
	p.SetRealm(clnt.Realm())

	// Set the parent dir
	p.SetParentDir(clnt.procdir)
	childProcdir := p.ProcDir

	db.DPrintf(db.PROCCLNT, "Spawn %v", p)
	if clnt.hasExited() != "" {
		db.DPrintf(db.PROCCLNT_ERR, "Spawn error called after Exited")
		db.DFatalf("Spawn error called after Exited")
	}

	if err := clnt.addChild(scheddIp, p, childProcdir, how); err != nil {
		return err
	}

	p.SetSpawnTime(time.Now())
	// If this is not a privileged proc, spawn it through procd.
	if how == HPROCD {
		req := &schedd.SpawnRequest{
			Realm:     clnt.Realm().String(),
			ProcProto: p.GetProto(),
		}
		res := &schedd.SpawnResponse{}
		err := pdc.RPC("Schedd.Spawn", req, res)
		if err != nil {
			db.DFatalf("Error spawn schedd: %v", err)
			return clnt.cleanupError(p.GetPid(), childProcdir, fmt.Errorf("Spawn error %v", err))
		}
	} else {
		// Make the proc's procdir
		err := clnt.MakeProcDir(p.GetPid(), p.ProcDir, p.IsPrivilegedProc())
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Err SpawnKernelProc MakeProcDir: %v", err)
		}
		// Create a semaphore to indicate a proc has started if this is a kernel
		// proc. Otherwise, procd will create the semaphore.
		childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, p.GetPid()))
		semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
		semStart.Init(0)
	}

	return nil
}

// Update the list of active procds.
func (clnt *ProcClnt) updateSchedds() {
	clnt.Lock()
	defer clnt.Unlock()

	// If we updated the list of active procds recently, return immediately. The
	// list will change at most as quickly as the realm resizes.
	if time.Since(clnt.lastProcdUpdate) < sp.Conf.Realm.RESIZE_INTERVAL && len(clnt.scheddIps) > 0 {
		db.DPrintf(db.PROCCLNT, "Update schedds too soon")
		return
	}
	clnt.lastProcdUpdate = time.Now()
	// Read the procd union dir.
	schedds, _, err := clnt.ReadDir(sp.SCHEDD)
	if err != nil {
		db.DFatalf("Error ReadDir procd: %v", err)
	}
	db.DPrintf(db.PROCCLNT, "Got schedds %v", schedds)
	// Alloc enough space for the list of procds, excluding the ws queue.
	clnt.scheddIps = make([]string, 0, len(schedds)-1)
	for _, schedd := range schedds {
		clnt.scheddIps = append(clnt.scheddIps, schedd.Name)
	}
}

func (clnt *ProcClnt) getScheddClnt(scheddIp string) *protdevclnt.ProtDevClnt {
	clnt.Lock()
	defer clnt.Unlock()

	// See if we already have a client for this procd.
	if pdc, ok := clnt.schedds[scheddIp]; ok {
		return pdc
	}
	pdc, err := protdevclnt.MkProtDevClnt(clnt.FsLib, path.Join(sp.SCHEDD, scheddIp))
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "Error make protdevclnt: %v", err)
		return nil
	}
	clnt.schedds[scheddIp] = pdc
	// Local procd is special: it has 2 entries, one under its IP and the other
	// under ~local.
	if scheddIp == "~local" {
		p, ok, err := clnt.ResolveUnion(path.Join(sp.SCHEDD, "~local"))
		if !ok || err != nil {
			// If ~local hasn't registered itself yet, this method should've bailed
			// out earlier.
			db.DFatalf("Couldn't resolve procd ~local: %v, %v, %v", p, ok, err)
		}
		scheddIp = path.Base(p)
		db.DPrintf(db.PROCCLNT, "Resolved ~local to %v", scheddIp)
		clnt.schedds[scheddIp] = pdc
	}
	return pdc
}

// Get the next procd to burst on.
func (clnt *ProcClnt) nextSchedd() string {
	clnt.Lock()
	defer clnt.Unlock()

	if len(clnt.scheddIps) == 0 {
		debug.PrintStack()
		db.DFatalf("Error: no schedds to spawn on")
	}

	clnt.burstOffset++
	return clnt.scheddIps[clnt.burstOffset%len(clnt.scheddIps)]
}

// ========== WAIT ==========

// Wait until a proc file is removed. Return an error if SetRemoveWatch returns
// an unreachable error.
func (clnt *ProcClnt) waitProcFileRemove(pid proc.Tpid, pn string) error {
	db.DPrintf(db.PROCCLNT, "%v set remove watch: %v", pid, pn)
	done := make(chan bool)
	err := clnt.SetRemoveWatch(pn, func(string, error) {
		done <- true
	})
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error waitStart SetRemoveWatch %v", err)
		if serr.IsErrUnreachable(err) {
			return err
		}
	} else {
		<-done
	}
	return nil
}

func (clnt *ProcClnt) waitStart(pid proc.Tpid) error {
	childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.PROCFILE_LINK))
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Can't get procip file: %v", err)
		return err
	}
	procfileLink := string(b)
	// Kernel procs will have empty proc file links.
	if procfileLink != "" {
		// Wait for the proc queue file to be removed. If the schedd holding this
		// queue becomes unreachable, return an error.
		if err := clnt.waitProcFileRemove(pid, procfileLink); err != nil {
			return err
		}
		// See if this proc was stolen by another schedd. If so, wait on the new
		// proc file link.
		b, err = clnt.GetFile(path.Join(childDir, proc.WS_LINK))
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "No ws link %v", pid)
		} else {
			wsLink := string(b)
			db.DPrintf(db.PROCCLNT_ERR, "Found ws link %v: %v", pid, wsLink)
			// Wait for the proc queue file (in the new, stealing schedd), to be
			// removed. If the schedd holding this queue becomes unreachable, return
			// an error.
			if err := clnt.waitProcFileRemove(pid, wsLink); err != nil {
				return err
			}
		}
	}
	db.DPrintf(db.PROCCLNT, "WaitStart %v %v", pid, childDir)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(childDir, proc.START_SEM))
	return semStart.Down()
}

// Parent calls WaitStart() to wait until the child proc has
// started. If the proc doesn't exist, return immediately.
func (clnt *ProcClnt) WaitStart(pid proc.Tpid) error {
	err := clnt.waitStart(pid)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "WaitStart %v %v", pid, err)
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
		db.DPrintf(db.PROCCLNT, "waitStart err %v", err)
	}

	db.DPrintf(db.PROCCLNT, "WaitExit %v", pid)

	// Make sure the child proc has exited.
	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(proc.GetChildProcDir(clnt.procdir, pid), proc.EXIT_SEM))
	if err := semExit.Down(); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Error WaitExit semExit.Down: %v", err)
		return nil, fmt.Errorf("Error semExit.Down: %v", err)
	}

	defer clnt.removeChild(pid)

	childDir := path.Dir(proc.GetChildProcDir(clnt.procdir, pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.EXIT_STATUS))
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Missing return status, procd must have crashed: %v, %v", pid, err)
		return nil, fmt.Errorf("Missing return status, procd must have crashed: %v", err)
	}

	status := &proc.Status{}
	if err := json.Unmarshal(b, status); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "waitexit unmarshal err %v", err)
		return nil, err
	}

	return status, nil
}

// Proc pid waits for eviction notice from procd.
func (clnt *ProcClnt) WaitEvict(pid proc.Tpid) error {
	db.DPrintf(db.PROCCLNT, "WaitEvict %v", pid)
	procdir := proc.PROCDIR
	semEvict := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EVICT_SEM))
	err := semEvict.Down()
	if err != nil {
		return fmt.Errorf("WaitEvict error %v", err)
	}
	db.DPrintf(db.PROCCLNT, "WaitEvict evicted %v", pid)
	return nil
}

// ========== STARTED ==========

// Proc pid marks itself as started.
func (clnt *ProcClnt) Started() error {
	db.DPrintf(db.PROCCLNT, "Started %v", clnt.pid)

	// Link self into parent dir
	if err := clnt.linkSelfIntoParentDir(); err != nil {
		db.DPrintf(db.PROCCLNT, "linkSelfIntoParentDir %v err %v", clnt.pid, err)
		return err
	}

	// Mark self as started
	semPath := path.Join(proc.PARENTDIR, proc.START_SEM)
	semStart := semclnt.MakeSemClnt(clnt.FsLib, semPath)
	err := semStart.Up()
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Started error %v %v", semPath, err)
	}
	// File may not be found if parent exited first or isn't reachable
	if err != nil && !serr.IsErrUnavailable(err) {
		return fmt.Errorf("Started error %v", err)
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
	db.DPrintf(db.PROCCLNT, "exited %v parent %v pid %v status %v", procdir, parentdir, pid, status)

	// will catch some unintended misuses: a proc calling exited
	// twice or procd calling exited twice.
	if clnt.setExited(pid) == pid {
		debug.PrintStack()
		db.DFatalf("Exited called after exited %v", procdir)
	}

	b, err := json.Marshal(status)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited marshal err %v", err)
		return err
	}
	// May return an error if parent already exited.
	fn := path.Join(parentdir, proc.EXIT_STATUS)
	if _, err := clnt.PutFile(fn, 0777, sp.OWRITE, b); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited error (parent already exited) MakeFile %v err %v", fn, err)
	}

	semExit := semclnt.MakeSemClnt(clnt.FsLib, path.Join(procdir, proc.EXIT_SEM))
	if err := semExit.Up(); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited semExit up error: %v, %v, %v", procdir, pid, err)
	}

	// clean myself up
	r := clnt.removeProc(procdir + "/")
	if r != nil {
		return fmt.Errorf("Exited error [%v] %v", procdir, r)
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
	db.DPrintf(db.PROCCLNT, "exited %v parent %v pid %v status %v", procdir, parentdir, pid, status)
	err := clnt.exited(procdir, parentdir, pid, status)
	if err != nil {
		// XXX maybe remove any state left of proc?
		db.DPrintf(db.PROCCLNT_ERR, "exited %v err %v", pid, err)
	}
	// If proc ran, but crashed before calling Started, the parent may block indefinitely. Stop this from happening by calling semStart.Up()
	semStart := semclnt.MakeSemClnt(clnt.FsLib, path.Join(parentdir, proc.START_SEM))
	semStart.Up()
}

// ========== EVICT ==========

// Notifies a proc that it will be evicted using Evict.
func (clnt *ProcClnt) evict(procdir string) error {
	db.DPrintf(db.PROCCLNT, "evict %v", procdir)
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
	procdir := path.Join(sp.PROCD, procdIp, proc.PIDS, pid.String())
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
