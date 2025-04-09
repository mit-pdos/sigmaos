package procmgr

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/proc/kproc"
	"sigmaos/util/perf"
)

func (mgr *ProcMgr) runProc(p *proc.Proc) error {
	db.DPrintf(db.PROCDMGR, "MSched.ProcMgr.runProc: %v time since spawn %v", p, time.Since(p.GetSpawnTime()))
	perf.LogSpawnLatency("MSched.ProcMgr.runProc", p.GetPid(), p.GetSpawnTime(), perf.TIME_NOT_SET)
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
		db.DPrintf(db.PROCDMGR, "runUserProc child err %v", childErr)
		return childErr
	} else if uprocErr != nil {
		// Unexpected error with uproc server.
		db.DPrintf(db.PROCDMGR, "runUserProc uprocsrv err %v", uprocErr)
		return uprocErr
	}
	return nil
}
