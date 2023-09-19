package procclnt

import (
	"fmt"
	"path"
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/proc"
	schedd "sigmaos/schedd/proto"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
)

func (clnt *ProcClnt) notify(method Tmethod, pid sp.Tpid, kernelID, semName string, how proc.Thow, skipSchedd bool) error {
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
			// Get the RPC client for the local schedd
			rpcc, err := clnt.getScheddClnt(kernelID)
			if err != nil {
				db.DFatalf("Err get schedd clnt rpcc %v", err)
			}
			req := &schedd.NotifyRequest{
				PidStr: pid.String(),
			}
			res := &schedd.NotifyResponse{}
			if err := rpcc.RPC("Schedd."+method.Verb(), req, res); err != nil {
				db.DFatalf("Error Schedd %v: %v", method.Verb(), err)
			}
		}
	} else {
		// If the proc was not spawned via schedd, notify via sem.
		if !isKProc(pid) {
			b := debug.Stack()
			db.DFatalf("Tried to %v non-kernel proc %v, stack:\n%v", method, pid, string(b))
		}
		kprocDir := proc.KProcDir(pid)
		db.DPrintf(db.PROCCLNT, "%v sem %v dir %v", method, pid, kprocDir)
		sem := semclnt.NewSemClnt(clnt.FsLib, path.Join(kprocDir, semName))
		err := sem.Up()
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Error %v: %v", method, err)
			return fmt.Errorf("%v error %v", method, err)
		}
	}
	return nil
}
