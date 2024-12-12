package procclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/util/coordination/semaphore"
	sp "sigmaos/sigmap"
)

func (clnt *ProcClnt) notify(method mschedclnt.Tmethod, pid sp.Tpid, kernelID string, semName string, how proc.Thow, status *proc.Status, skipMSched bool) error {
	db.DPrintf(db.PROCCLNT, "%v %v", method, pid)
	defer db.DPrintf(db.PROCCLNT, "%v done %v", method, pid)

	if how == proc.HMSCHED {
		if skipMSched {
			// Skip notifying via msched. Currently, this only happens when the proc
			// crashes and msched calls ExitedCrashed on behalf of the proc.  MSched
			// will take care of calling Exited locally, so no need to RPC (itself).
			//
			// Do nothing
		} else {
			// If the proc was spawned via msched, notify via RPC.
			db.DPrintf(db.PROCCLNT, "%v %v RPC", method, pid)
			if err := clnt.mschedclnt.Notify(method, kernelID, pid, status); err != nil {
				db.DPrintf(db.PROCCLNT_ERR, "Error msched %v: %v", method, err)
				return err
			}
		}
	} else {
		// If the proc was not spawned via msched, notify via sem.
		kprocDir := proc.KProcDir(pid)
		db.DPrintf(db.PROCCLNT, "%v sem %v dir %v", method, pid, kprocDir)
		sem := semaphore.NewSemaphore(clnt.FsLib, filepath.Join(kprocDir, semName))
		err := sem.Up()
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Error %v: %v", method, err)
			return err
		}
	}
	return nil
}
