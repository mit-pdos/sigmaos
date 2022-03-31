package procd

import (
	"encoding/json"
	"path"

	db "ulambda/debug"
	"ulambda/proc"
)

func (pd *Procd) readRunqProc(procdPath string, queueName string, pid string) (*proc.Proc, error) {
	b, err := pd.GetFile(path.Join(procdPath, queueName, pid))
	if err != nil {
		return nil, err
	}
	p := proc.MakeEmptyProc()
	err = json.Unmarshal(b, p)
	if err != nil {
		db.DFatalf("Error Unmarshal in Procd.readProc: %v", err)
		return nil, err
	}
	return p, nil
}

func (pd *Procd) claimProc(procdPath string, queueName string, p *proc.Proc) bool {
	err := pd.Remove(path.Join(procdPath, queueName, p.Pid.String()))
	if err != nil {
		return false
	}
	return true
}
