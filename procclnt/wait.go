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

// Wait for an event. Method must be one of "Exit", "Evict", or "Start"
func (clnt *ProcClnt) wait(method Tmethod, pid sp.Tpid, kernelID, semName string, how proc.Thow) error {
	db.DPrintf(db.PROCCLNT, "Wait%v %v how %v", method, pid, how)
	defer db.DPrintf(db.PROCCLNT, "Wait%v done %v", method, pid)

	// If spawned via schedd, wait via RPC.
	if how == proc.HSCHEDD {
		// RPC the schedd this proc was spawned on to wait.
		db.DPrintf(db.PROCCLNT, "Wait%v %v RPC", method, pid)
		rpcc, err := clnt.getScheddClnt(kernelID)
		if err != nil {
			db.DFatalf("Err get schedd clnt rpcc %v", err)
		}
		req := &schedd.WaitRequest{
			PidStr: pid.String(),
		}
		res := &schedd.WaitResponse{}
		if err := rpcc.RPC("Schedd.Wait"+method.String(), req, res); err != nil {
			db.DFatalf("Error Schedd Wait%v: %v", method, err)
		}
	} else {
		if !isKProc(pid) {
			b := debug.Stack()
			db.DFatalf("Tried to Wait%v non-kernel proc %v, stack:\n%v", method, pid, string(b))
		}
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
