package procclnt

import (
	"fmt"
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
)

// Wait for an event. Method must be one of "Exit", "Evict", or "Start"
func (clnt *ProcClnt) wait(method scheddclnt.Tmethod, pid sp.Tpid, kernelID, semName string, how proc.Thow) error {
	db.DPrintf(db.PROCCLNT, "Wait%v %v how %v", method, pid, how)
	defer db.DPrintf(db.PROCCLNT, "Wait%v done %v", method, pid)

	// If spawned via schedd, wait via RPC.
	if how == proc.HSCHEDD {
		// RPC the schedd this proc was spawned on to wait.
		db.DPrintf(db.PROCCLNT, "Wait%v %v RPC", method, pid)
		err := clnt.scheddclnt.Wait(method, kernelID, pid)
		if err != nil {
			db.DFatalf("Error Schedd Wait%v: %v", method, err)
		}
	} else {
		// If not spawned via schedd, wait via semaphore.
		kprocDir := proc.KProcDir(pid)
		db.DPrintf(db.PROCCLNT, "Wait%v sem %v dir %v", method, pid, kprocDir)
		sem := semclnt.NewSemClnt(clnt.FsLib, path.Join(kprocDir, semName))
		err := sem.Down()
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Wait%v error %v", method, err)
			return fmt.Errorf("Wait%v error %v", method, err)
		}
	}
	return nil
}
