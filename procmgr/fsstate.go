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

// Set up a proc's state in the realm.
func (mgr *ProcMgr) setupProcState(p *proc.Proc) {
	mgr.addRunningProc(p)
	// Set up the directory to cache proc binaries for this realm.
	mgr.setupUserBinCache(p)
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
