package procmgr

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/kproc"
	"sigmaos/proc"
)

func (mgr *ProcMgr) runProc(p *proc.Proc) {
	db.DPrintf(db.PROCMGR, "Procd run: %v\nQueueing delay: %v", p, time.Since(p.GetSpawnTime()))
	var err error
	if p.IsPrivileged() {
		err = mgr.runPrivilegedProc(p)
	} else {
		err = mgr.runUserProc(p)
	}
	if err != nil {
		mgr.procCrashed(p, err)
	}
}

func (mgr *ProcMgr) runPrivilegedProc(p *proc.Proc) error {
	cmd, err := kproc.RunKernelProc(mgr.rootsc.ProcEnv().GetLocalIP(), p, nil)
	if err != nil {
		db.DFatalf("Couldn't start privileged proc: %v", err)
		return err
	}
	return cmd.Wait()
}

func (mgr *ProcMgr) runUserProc(p *proc.Proc) error {
	if uprocErr, childErr := mgr.updm.RunUProc(p); childErr != nil {
		return childErr
	} else if uprocErr != nil {
		// Unexpected error with uproc server.
		db.DPrintf(db.PROCMGR, "runUserProc uproc err %v", uprocErr)
		return uprocErr
	}
	return nil
}
