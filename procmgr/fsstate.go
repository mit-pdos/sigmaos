package procmgr

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

//
// Proc state management in the realm namespace.
//

// Post a proc file in the local queue.
func (mgr *ProcMgr) postProcInQueue(p *proc.Proc) {
	if _, err := mgr.mfs.Create(path.Join(sp.QUEUE, p.GetPid().String()), 0777, sp.OWRITE, sp.NoLeaseId); err != nil {
		db.DFatalf("Error create %v: %v", p.GetPid(), err)
	}
}

// Set up a proc's state in the realm.
func (mgr *ProcMgr) setupProcState(p *proc.Proc) {
	mgr.addRunningProc(p)
	// Set up the directory to cache proc binaries for this realm.
	mgr.setupUserBinCache(p)
	// Release the parent proc, which may be waiting for removal of the proc
	// queue file in WaitStart.
	if err := mgr.rootsc.Remove(path.Join(sp.SCHEDD, p.GetKernelID(), sp.QUEUE, p.GetPid().String())); err != nil {
		// Check if the proc was stoelln from another schedd.
		stolen := p.GetKernelID() != mgr.kernelId
		if stolen {
			// May return an error if the schedd stolen from crashes.
			db.DPrintf(db.PROCMGR_ERR, "Error remove schedd queue file [%v]: %v", p.GetKernelID(), err)
		} else {
			// Removing from self should always succeed.
			db.DFatalf("Error remove schedd queue file: %v", err)
		}
	}
	// Make the proc's procdir
	if err := mgr.rootsc.NewProcDir(p.GetPid(), p.GetProcDir(), p.IsPrivileged(), proc.HSCHEDD); err != nil {
		db.DPrintf(db.PROCMGR_ERR, "Err procmgr NewProcDir: %v\n", err)
	}
}

func (mgr *ProcMgr) teardownProcState(p *proc.Proc) {
	mgr.removeRunningProc(p)
}

// Set up state to notify parent that a proc crashed.
func (mgr *ProcMgr) procCrashed(p *proc.Proc, err error) {
	db.DPrintf(db.PROCMGR_ERR, "Proc %v finished with error: %v", p, err)
	mgr.getSigmaClnt(p.GetRealm()).ExitedCrashed(p.GetPid(), p.GetProcDir(), p.GetParentDir(), proc.NewStatusErr(err.Error(), nil), p.GetHow())
}

// Register a proc as running.
func (mgr *ProcMgr) addRunningProc(p *proc.Proc) {
	mgr.Lock()
	defer mgr.Unlock()

	mgr.running[p.GetPid()] = p
	_, err := mgr.rootsc.PutFile(path.Join(sp.SCHEDD, mgr.kernelId, sp.RUNNING, p.GetPid().String()), 0777, sp.OREAD|sp.OWRITE, p.MarshalJson())
	if err != nil {
		db.DFatalf("Error PutFile in running queue: %v", err)
	}
}

// Unregister a proc which has finished running.
func (mgr *ProcMgr) removeRunningProc(p *proc.Proc) {
	mgr.Lock()
	defer mgr.Unlock()

	delete(mgr.running, p.GetPid())
	if err := mgr.mfs.Remove(path.Join(sp.RUNNING, p.GetPid().String())); err != nil {
		db.DFatalf("Error Remove from running queue: %v", err)
	}
}
