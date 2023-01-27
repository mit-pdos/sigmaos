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

// Set up a proc's state in the realm.
func (mgr *ProcMgr) setupProcState(p *proc.Proc) {
	mgr.addRunningProc(p)
	// Create an ephemeral semaphore for the parent proc to wait on.
	//	sclnt := pd.getSigmaClnt(p.GetRealm())
	semPath := path.Join(p.ParentDir, proc.START_SEM)
	semStart := semclnt.MakeSemClnt(mgr.getSigmaClnt(p.GetRealm()).FsLib, semPath)
	if err := semStart.Init(sp.DMTMP); err != nil {
		db.DPrintf(db.PROCMGR_ERR, "Error creating start semaphore path:%v err:%v", semPath, err)
	}
	db.DPrintf(db.PROCMGR, "Sem init done: %v", p)
	// Release the parent proc, which may be waiting for removal of the proc
	// queue file in WaitStart.
	if err := mgr.rootsc.Remove(path.Join(sp.SCHEDD, "~local", sp.QUEUE, p.GetPid().String())); err != nil {
		db.DFatalf("Error remove schedd queue file: %v", err)
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
	err := mgr.rootsc.Remove(path.Join(sp.SCHEDD, "~local", sp.RUNNING, p.GetPid().String()))
	if err != nil {
		db.DFatalf("Error Remove from running queue: %v", err)
	}
}
