package electclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
)

//
// Library to acquire leadership
//

type ElectClnt struct {
	*fslib.FsLib
	path string // pathname for the leader-election file
	perm sp.Tperm
	mode sp.Tmode
}

func MakeElectClnt(fsl *fslib.FsLib, path string, perm sp.Tperm) *ElectClnt {
	e := &ElectClnt{}
	e.path = path
	e.FsLib = fsl
	e.perm = perm
	return e
}

func (e *ElectClnt) AcquireLeadership(b []byte) error {
	fd, err := e.Create(e.path, e.perm|sp.DMTMP, sp.OWRITE|sp.OWATCH)
	if err != nil {
		db.DPrintf("LEADER_ERR", "Create %v err %v", e.path, err)
		return err
	}
	if _, err := e.WriteV(fd, b); err != nil {
		return err
	}
	e.Close(fd)
	return nil
}

func (e *ElectClnt) ReleaseLeadership() error {
	err := e.Remove(e.path)
	if err != nil {
		db.DPrintf("LEADER_ERR", "Remove %v err %v", e.path, err)
		return err
	}
	return nil
}
