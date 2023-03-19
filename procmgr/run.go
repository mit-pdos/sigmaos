package procmgr

import (
	"os"
	"os/exec"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
)

func (mgr *ProcMgr) runProc(p *proc.Proc) {
	db.DPrintf(db.PROCMGR, "Procd run: %v\nQueueing delay: %v", p, time.Since(p.GetSpawnTime()))
	var err error
	if p.IsPrivilegedProc() {
		err = mgr.runPrivilegedProc(p)
	} else {
		err = mgr.runUserProc(p)
	}
	if err != nil {
		mgr.procCrashed(p, err)
	}
}

func (mgr *ProcMgr) runPrivilegedProc(p *proc.Proc) error {
	cmd := exec.Command(p.Program, p.Args...)
	cmd.Env = p.GetEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
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
		db.DFatalf("Error setting up uprocd: %v", uprocErr)
		return uprocErr
	}
	return nil
}
