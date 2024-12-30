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
	// Make the proc's procdir if this is a kernel proc.
	if p.IsPrivileged() {
		if err := mgr.rootsc.MakeProcDir(p.GetPid(), p.GetProcDir()); err != nil {
			db.DPrintf(db.PROCDMGR_ERR, "Err procmgr MakeProcDir: %v\n", err)
			db.DFatalf("Err MakeProcDir: %v", err)
		}
	}
}
