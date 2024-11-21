package procclnt

import (
	"fmt"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func (clnt *ProcClnt) getExitStatus(pid sp.Tpid, how proc.Thow) (*proc.Status, error) {
	status := clnt.cs.GetExitStatus(pid)
	if how == proc.HMSCHED {
		// Status will be cached in the child state struct
		return status, nil
	} else {
		// Status must be read from kproc dir
		kprocDir := proc.KProcDir(pid)
		var b []byte
		b, err := clnt.GetFile(filepath.Join(kprocDir, proc.EXIT_STATUS))
		if err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "Missing return status, schedd must have crashed: %v, %v", pid, err)
			return nil, fmt.Errorf("Missing return status, schedd must have crashed: %v", err)
		}
		status = proc.NewStatusFromBytes(b)
	}
	return status, nil
}

func (clnt *ProcClnt) writeExitStatus(pid sp.Tpid, kernelID string, status *proc.Status, how proc.Thow) error {
	if how == proc.HMSCHED {
		// Do nothing... exit status will be set by Notify RPC to schedd.
	} else {
		// Must write status to kproc dir
		b := status.Marshal()
		// May return an error if parent already exited.
		kprocDir := proc.KProcDir(pid)
		fn := filepath.Join(kprocDir, proc.EXIT_STATUS)
		db.DPrintf(db.PROCCLNT, "writeExitStatus via named")
		defer db.DPrintf(db.PROCCLNT, "writeExitStatus done via named")
		if _, err := clnt.PutFile(fn, 0777, sp.OWRITE, b); err != nil {
			db.DPrintf(db.PROCCLNT_ERR, "exited error (parent already exited) NewFile %v err %v", fn, err)
		}
	}
	return nil
}
