package procclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
)

func (clnt *ProcClnt) notify(method scheddclnt.Tmethod, pid sp.Tpid, kernelID, semName string, how proc.Thow, status *proc.Status, skipSchedd bool) error {
	db.DPrintf(db.PROCCLNT, "%v %v", method, pid)
	defer db.DPrintf(db.PROCCLNT, "%v done %v", method, pid)

	if how == proc.HSCHEDD {
		if skipSchedd {
			// Skip notifying via schedd. Currently, this only happens when the proc
			// crashes and schedd calls ExitedCrashed on behalf of the proc.  Schedd
			// will take care of calling Exited locally, so no need to RPC (itself).
			//
			// Do nothing
		} else {
			// If the proc was spawned via schedd, notify via RPC.
			db.DPrintf(db.PROCCLNT, "%v %v RPC", method, pid)
			if err := clnt.scheddclnt.Notify(method, kernelID, pid, status); err != nil {
				db.DPrintf(db.PROCCLNT_ERR, "Error schedd %v: %v", method, err)
				return err
			}
		}
	} else {
		// If the proc was not spawned via schedd, notify via sem.
		kprocDir := proc.KProcDir(pid)
		db.DPrintf(db.PROCCLNT, "%v sem %v dir %v", method, pid, kprocDir)
		sem := semclnt.NewSemClnt(clnt.FsLib, filepath.Join(kprocDir, semName))
		err := sem.Up()
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Error %v: %v", method, err)
			return err
		}
	}
	return nil
}
