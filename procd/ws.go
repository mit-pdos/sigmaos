package procd

import (
	"encoding/json"
	"log"
	"path"

	np "ulambda/ninep"
	"ulambda/proc"
)

func (pd *Procd) readRemoteRunq(procdPath string, queueName string) ([]*np.Stat, error) {
	return pd.GetDir(path.Join(procdPath, queueName))
}

func (pd *Procd) readRemoteRunqProc(procdPath string, queueName string, pid string) (*proc.Proc, error) {
	b, err := pd.GetFile(path.Join(procdPath, queueName, pid))
	if err != nil {
		return nil, err
	}
	p := proc.MakeEmptyProc()
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("FATAL Error Unmarshal in Procd.readRemoteProc: %v", err)
		return nil, err
	}
	return p, nil
}

func (pd *Procd) claimRemoteProc(procdPath string, queueName string, p *proc.Proc) bool {
	err := pd.Remove(path.Join(procdPath, queueName, p.Pid.String()))
	if err != nil {
		return false
	}
	return true
}
