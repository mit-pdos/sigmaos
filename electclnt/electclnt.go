package electclnt

import (
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type ElectClnt struct {
	*fslib.FsLib
	path string // pathname for the leader-election file
	fd   int
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

func (e *ElectClnt) AcquireLeadership() error {
	fd, err := e.Create(e.path, e.perm|np.DMTMP, np.OWRITE|np.OWATCH)
	if err != nil {
		db.DLPrintf("LEADER_ERR", "Create %v err %v", e.path, err)
		return err
	}
	e.fd = fd
	return nil
}

func (e *ElectClnt) ReleaseLeadership() error {
	e.Close(e.fd)
	err := e.Remove(e.path)
	if err != nil {
		db.DLPrintf("LEADER_ERR", "Remove %v err %v", e.path, err)
		return err
	}
	return nil
}
