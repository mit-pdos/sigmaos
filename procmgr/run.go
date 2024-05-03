package procmgr

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/kproc"
	"sigmaos/proc"
)

func (mgr *ProcMgr) runProc(p *proc.Proc) error {
	db.DPrintf(db.PROCMGR, "Procd run: %v time since spawn %v", p, time.Since(p.GetSpawnTime()))
	var err error
	if p.IsPrivileged() {
		err = mgr.runPrivilegedProc(p)
	} else {
		err = mgr.runUserProc(p)
	}
	return err
}

func (mgr *ProcMgr) runPrivilegedProc(p *proc.Proc) error {
	cmd, err := kproc.RunKernelProc(mgr.rootsc.ProcEnv().GetInnerContainerIP(), p, nil)
	if err != nil {
		db.DPrintf(db.ERROR, "Couldn't start privileged proc: %v", err)
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
