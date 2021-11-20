package procd

import (
	"encoding/json"
	"log"
	"path"

	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
)

func (pd *Procd) readRemoteRunq(procdPath string) ([]*np.Stat, error) {
	return pd.ReadDir(path.Join(procdPath, named.PROCD_RUNQ))
}

func (pd *Procd) readRemoteRunqProc(procdPath string, pid string) (*proc.Proc, error) {
	b, err := pd.ReadFile(path.Join(procdPath, named.PROCD_RUNQ, pid))
	if err != nil {
		return nil, err
	}
	p := proc.MakeEmptyProc()
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Error Unmarshal in Procd.readRemoteProc: %v", err)
		return nil, err
	}
	return p, nil
}

func (pd *Procd) claimRemoteProc(procdPath string, p *proc.Proc) bool {
	err := pd.Remove(path.Join(procdPath, named.PROCD_RUNQ, p.Pid))
	if err != nil {
		return false
	}
	return true
}
