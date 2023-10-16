package procmgr

import (
	db "sigmaos/debug"
	"sigmaos/proc"
)

//
// Proc state management in the realm namespace.
//

// Set up a proc's state in the realm.
func (mgr *ProcMgr) setupProcState(p *proc.Proc) {
	// Set up the directory to cache proc binaries for this realm.
	mgr.setupUserBinCache(p)
	// Make the proc's procdir if this is a kernel proc. This will be done lazily
	// for user procs.
	if p.IsPrivileged() {
		if err := mgr.rootsc.MakeProcDir(p.GetPid(), p.GetProcDir(), p.IsPrivileged(), proc.HSCHEDD); err != nil {
			db.DPrintf(db.PROCMGR_ERR, "Err procmgr MakeProcDir: %v\n", err)
		}
	}
}

// Set up state to notify parent that a proc crashed.
func (mgr *ProcMgr) procCrashed(p *proc.Proc, err error) {
	// Mark the proc as exited due to a crash, and record the error exit status.
	mgr.pstate.exited(p.GetPid(), proc.NewStatusErr(err.Error(), nil).Marshal())
	db.DPrintf(db.PROCMGR_ERR, "Proc %v finished with error: %v", p, err)
	mgr.getSigmaClnt(p.GetRealm()).ExitedCrashed(p.GetPid(), p.GetProcDir(), p.GetParentDir(), proc.NewStatusErr(err.Error(), nil), p.GetHow())
}
