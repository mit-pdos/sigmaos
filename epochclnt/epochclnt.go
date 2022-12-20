package epochclnt

import (
	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
)

//
// Library for ops on the epoch file (i.e., a regular file that
// contains an epoch number).
//

type EpochClnt struct {
	*fslib.FsLib
	path string
	perm sp.Tperm
}

func MakeEpochClnt(fsl *fslib.FsLib, leaderfn string, perm sp.Tperm) *EpochClnt {
	ec := &EpochClnt{}
	ec.FsLib = fsl
	ec.path = leaderfn + "-epoch"
	ec.perm = perm
	return ec
}

func (ec *EpochClnt) Name() string {
	return ec.path
}

func (ec *EpochClnt) AdvanceEpoch() (sp.Tepoch, error) {
	fd, err := ec.CreateOpen(ec.path, ec.perm&0xFF, sp.ORDWR)
	if err != nil {
		db.DPrintf(db.EPOCHCLNT_ERR, "CreateOpen %v err %v", ec.path, err)
		return sp.NoEpoch, err
	}
	defer ec.Close(fd)
	b, err := ec.Read(fd, 100)
	if err != nil {
		db.DPrintf(db.EPOCHCLNT_ERR, "Read %v err %v", ec.path, err)
		return sp.NoEpoch, err
	}
	n := sp.Tepoch(0)
	if len(b) > 0 {
		n, err = sp.String2Epoch(string(b))
		if err != nil {
			db.DPrintf(db.EPOCHCLNT_ERR, "String2Epoch %v err %v", string(b), err)
			return sp.NoEpoch, err
		}
	}
	n += 1
	if err := ec.Seek(fd, 0); err != nil {
		db.DPrintf(db.EPOCHCLNT_ERR, "Seek %v err %v", fd, err)
		return sp.NoEpoch, err
	}

	db.DPrintf(db.EPOCHCLNT, "AdvanceEpoch %v %v", ec.path, n)

	_, err = ec.WriteV(fd, []byte(n.String()))
	if err != nil {
		db.DPrintf(db.EPOCHCLNT_ERR, "Write %v err %v", ec.path, err)
		return sp.NoEpoch, err
	}
	return n, nil
}

func (ec *EpochClnt) ReadEpoch() (sp.Tepoch, error) {
	b, err := ec.GetFile(ec.path)
	if err != nil {
		return sp.NoEpoch, err
	}
	e, err := sp.String2Epoch(string(b))
	if err != nil {
		return sp.NoEpoch, err
	}
	return e, nil
}

func (ec *EpochClnt) GetFence(epoch sp.Tepoch) (*sp.Tfence, error) {
	f := sp.MakeFenceNull()
	fd, err := ec.Open(ec.path, sp.OWRITE)
	if err != nil {
		db.DPrintf(db.EPOCHCLNT_ERR, "Open %v err %v", ec.path, err)
		return f, err
	}
	defer ec.Close(fd)

	b, err := ec.ReadV(fd, 100)
	if err != nil {
		db.DPrintf(db.EPOCHCLNT_ERR, "Read %v err %v", ec.path, err)
		return f, err
	}
	if string(b) != epoch.String() {
		db.DPrintf(db.EPOCHCLNT_ERR, "Epoch mismatch %v err %v", ec.path, err)
		return f, fcall.MkErr(fcall.TErrStale, "newer epoch: "+string(b))
	}
	qid, err := ec.Qid(fd)
	if err != nil {
		db.DPrintf(db.EPOCHCLNT_ERR, "Qid %v err %v", fd, err)
		return sp.MakeFenceNull(), err
	}
	f.Epoch = uint64(epoch)
	f.Fenceid.Path = uint64(qid.Path)
	return f, nil

}
