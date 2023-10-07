package procclnt

import (
	"encoding/json"
	"fmt"
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func (clnt *ProcClnt) getExitStatus(pid sp.Tpid, how proc.Thow) (*proc.Status, error) {
	childDir := path.Dir(proc.GetChildProcDir(proc.PROCDIR, pid))
	b, err := clnt.GetFile(path.Join(childDir, proc.EXIT_STATUS))
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "Missing return status, schedd must have crashed: %v, %v", pid, err)
		return nil, fmt.Errorf("Missing return status, schedd must have crashed: %v", err)
	}

	status := &proc.Status{}
	if err := json.Unmarshal(b, status); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "waitexit unmarshal err %v", err)
		return nil, err
	}
	return status, nil
}

func (clnt *ProcClnt) writeExitStatus(parentdir string, status *proc.Status, how proc.Thow) error {
	b, err := json.Marshal(status)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited marshal err %v", err)
		return err
	}
	// May return an error if parent already exited.
	fn := path.Join(parentdir, proc.EXIT_STATUS)
	if _, err := clnt.PutFile(fn, 0777, sp.OWRITE, b); err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "exited error (parent already exited) NewFile %v err %v", fn, err)
	}
	return nil
}
