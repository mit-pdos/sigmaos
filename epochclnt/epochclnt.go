package epochclnt

import (
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type EpochClnt struct {
	*fslib.FsLib
	path string
	perm np.Tperm
}

func MakeEpochClnt(fsl *fslib.FsLib, leaderfn string, perm np.Tperm) *EpochClnt {
	ec := &EpochClnt{}
	ec.FsLib = fsl
	ec.path = leaderfn + "-epoch"
	ec.perm = perm
	return ec
}

func (ec *EpochClnt) Name() string {
	return ec.path
}

func (ec *EpochClnt) AdvanceEpoch() (np.Tepoch, error) {
	fd, err := ec.CreateOpen(ec.path, ec.perm&0xFF, np.ORDWR)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "CreateOpen %v err %v", ec.path, err)
		return np.NoEpoch, err
	}
	defer ec.Close(fd)
	b, err := ec.Read(fd, 100)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "Read %v err %v", ec.path, err)
		return np.NoEpoch, err
	}
	n := np.Tepoch(0)
	if len(b) > 0 {
		n, err = np.String2Epoch(string(b))
		if err != nil {
			db.DLPrintf("EPOCHCLNT_ERR", "String2Epoch %v err %v", string(b), err)
			return np.NoEpoch, err
		}
	}
	n += 1
	if err := ec.Seek(fd, 0); err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "Seek %v err %v", fd, err)
		return np.NoEpoch, err
	}

	db.DLPrintf("EPOCHCLNT", "AdvanceEpoch %v %v", ec.path, n)

	_, err = ec.WriteV(fd, []byte(n.String()))
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "Write %v err %v", ec.path, err)
		return np.NoEpoch, err
	}
	return n, nil
}

func (ec *EpochClnt) GetEpoch() (np.Tepoch, error) {
	b, err := ec.GetFile(ec.path)
	if err != nil {
		return np.NoEpoch, err
	}
	e, err := np.String2Epoch(string(b))
	if err != nil {
		return np.NoEpoch, err
	}
	return e, nil
}

func (ec *EpochClnt) GetFence(epoch np.Tepoch) (np.Tfence1, error) {
	f := np.Tfence1{}
	fd, err := ec.Open(ec.path, np.OWRITE)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "Open %v err %v", ec.path, err)
		return f, err
	}
	defer ec.Close(fd)

	b, err := ec.ReadV(fd, 100)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "Read %v err %v", ec.path, err)
		return f, err
	}
	if string(b) != epoch.String() {
		db.DLPrintf("EPOCHCLNT_ERR", "Epoch mismatch %v err %v", ec.path, err)
		return f, np.MkErr(np.TErrVersion, "newer epoch: "+string(b))
	}
	qid, err := ec.Qid(fd)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "Qid %v err %v", fd, err)
		return np.Tfence1{}, err
	}
	f.Epoch = epoch
	f.FenceId.Path = qid.Path
	return f, nil

}
