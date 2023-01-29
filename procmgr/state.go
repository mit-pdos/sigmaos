package procmgr

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
)

//
// Proc state management in the realm namespace.
//

// Post a proc file in the local queue.
func (mgr *ProcMgr) postProcInQueue(p *proc.Proc) {
	if _, err := mgr.mfs.Create(path.Join(sp.QUEUE, p.GetPid().String()), 0777, sp.OWRITE); err != nil {
		db.DFatalf("Error create %v: %v", p.GetPid(), err)
	}
}

// Create an ephemeral "Started" semaphore. Must be ephemeral so parent procs can detect schedd crashes.
func (mgr *ProcMgr) createStartedSem(p *proc.Proc) (*semclnt.SemClnt, error) {
	semPath := path.Join(p.ParentDir, proc.START_SEM)
	semStart := semclnt.MakeSemClnt(mgr.getSigmaClnt(p.GetRealm()).FsLib, semPath)
	var err error
	if err = semStart.Init(sp.DMTMP); err == nil {
		db.DPrintf(db.PROCMGR, "Sem init done: %v", p)
	}
	return semStart, err
}

// Set up a proc's state in the realm.
func (mgr *ProcMgr) setupProcState(p *proc.Proc) {
	mgr.addRunningProc(p)
	// Create started semaphore, if the proc was not stolen. If the proc was
	// stolen, the started semaphore would have been created as part of the
	// stealing process.
	if _, err := mgr.createStartedSem(p); err != nil {
		db.DPrintf(db.PROCMGR_ERR, "Error creating start semaphore path:%v err:%v", path.Join(p.ParentDir, proc.START_SEM), err)
	}
	// Release the parent proc, which may be waiting for removal of the proc
	// queue file in WaitStart.
	if err := mgr.rootsc.Remove(path.Join(sp.SCHEDD, p.ScheddIp, sp.QUEUE, p.GetPid().String())); err != nil {
		// Check if the proc was stoelln from another schedd.
		stolen := p.ScheddIp != mgr.mfs.MyAddr()
		if stolen {
			// May return an error if the schedd stolen from crashes.
			db.DPrintf(db.PROCMGR_ERR, "Error remove schedd queue file [%v]: %v", p.ScheddIp, err)
		} else {
			// Removing from self should always succeed.
			db.DFatalf("Error remove schedd queue file: %v", err)
		}
	}
	// Make the proc's procdir
	if err := mgr.rootsc.MakeProcDir(p.GetPid(), p.ProcDir, p.IsPrivilegedProc()); err != nil {
		db.DPrintf(db.PROCMGR_ERR, "Err procmgr MakeProcDir: %v\n", err)
	}
}

func (mgr *ProcMgr) teardownProcState(p *proc.Proc) {
	mgr.removeRunningProc(p)
}

// Set up state to notify parent that a proc crashed.
func (mgr *ProcMgr) procCrashed(p *proc.Proc, err error) {
	db.DPrintf(db.PROCMGR_ERR, "Proc %v finished with error: %v", p, err)
	procclnt.ExitedProcd(mgr.getSigmaClnt(p.GetRealm()).FsLib, p.GetPid(), p.ProcDir, p.ParentDir, proc.MakeStatusErr(err.Error(), nil))
}

// Register a proc as running.
func (mgr *ProcMgr) addRunningProc(p *proc.Proc) {
	mgr.Lock()
	defer mgr.Unlock()

	// XXX Write package to expose running map as a dir.
	mgr.running[p.GetPid()] = p
	_, err := mgr.rootsc.PutFile(path.Join(sp.SCHEDD, "~local", sp.RUNNING, p.GetPid().String()), 0777, sp.OREAD|sp.OWRITE, p.Marshal())
	if err != nil {
		db.DFatalf("Error PutFile in running queue: %v", err)
	}
}

// Unregister a proc which has finished running.
func (mgr *ProcMgr) removeRunningProc(p *proc.Proc) {
	mgr.Lock()
	defer mgr.Unlock()

	// XXX Write package to expose running map as a dir.
	delete(mgr.running, p.GetPid())
	if err := mgr.mfs.Remove(path.Join(sp.RUNNING, p.GetPid().String())); err != nil {
		db.DFatalf("Error Remove from running queue: %v", err)
	}
}

func getWSQueue(t proc.Ttype) string {
	var q string
	switch t {
	case proc.T_LC:
		q = sp.WS_RUNQ_LC
	case proc.T_BE:
		q = sp.WS_RUNQ_BE
	default:
		db.DFatalf("Unrecognized proc type: %v", t)
	}
	return q
}

func (mgr *ProcMgr) removeWSLink(p *proc.Proc) {
	mgr.rootsc.Remove(path.Join(getWSQueue(p.GetType()), p.GetPid().String()))
}

func (mgr *ProcMgr) createWSLink(p *proc.Proc) {
	if _, err := mgr.rootsc.PutFile(path.Join(getWSQueue(p.GetType()), p.GetPid().String()), 0777, sp.OWRITE, p.Marshal()); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
}
