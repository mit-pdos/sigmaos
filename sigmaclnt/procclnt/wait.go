package procclnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/util/coordination/barrier"
	sp "sigmaos/sigmap"
)

// Wait for an event. Method must be one of "Exit", "Evict", or "Start"
func (clnt *ProcClnt) wait(method mschedclnt.Tmethod, pid sp.Tpid, mschedID string, pseqno *proc.ProcSeqno, semName string, how proc.Thow) (*proc.Status, error) {
	db.DPrintf(db.PROCCLNT, "Wait%v %v how %v seqno %v", method, pid, how, pseqno)
	defer db.DPrintf(db.PROCCLNT, "Wait%v done %v, seqno %v", method, pid, pseqno)

	var status *proc.Status
	// If spawned via msched, wait via RPC.
	if how == proc.HMSCHED {
		// RPC the msched this proc was spawned on to wait.
		db.DPrintf(db.PROCCLNT, "Wait%v %v RPC", method, pid)
		var err error
		status, err = clnt.mschedclnt.Wait(method, mschedID, pseqno, pid)
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Error MSched Wait%v: %v", method, err)
			return nil, err
		}
	} else {
		// If not spawned via msched, wait via semaphore.
		kprocDir := proc.KProcDir(pid)
		db.DPrintf(db.PROCCLNT, "Wait%v sem %v dir %v", method, pid, kprocDir)
		sem := barrier.NewBarrier(clnt.FsLib, filepath.Join(kprocDir, semName))
		err := sem.Down()
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Wait%v error %v", method, err)
			return nil, err
		}
	}
	return status, nil
}
