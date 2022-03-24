package electclnt

import (
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

//
// Library to acquire leadership
//

type ElectClnt struct {
	*fslib.FsLib
	path string // pathname for the leader-election file
	perm np.Tperm
	mode np.Tmode
}

func MakeElectClnt(fsl *fslib.FsLib, path string, perm np.Tperm) *ElectClnt {
	e := &ElectClnt{}
	e.path = path
	e.FsLib = fsl
	e.perm = perm
	return e
}

func (e *ElectClnt) AcquireLeadership(leader []byte) error {
	fd, err := e.Create(e.path, e.perm|np.DMTMP, np.OWRITE|np.OWATCH)
	if err != nil {
		db.DLPrintf("LEADER_ERR", "Create %v err %v", e.path, err)
		return err
	}
	if _, err := e.WriteV(fd, leader); err != nil {
		return err
	}
	e.Close(fd)
	return nil
}

func (e *ElectClnt) ReleaseLeadership() error {
	err := e.Remove(e.path)
	if err != nil {
		db.DLPrintf("LEADER_ERR", "Remove %v err %v", e.path, err)
		return err
	}
	return nil
}
