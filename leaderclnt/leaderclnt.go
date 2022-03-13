package leaderclnt

import (
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type LeaderClnt struct {
	path string // pathname for the leader-election file
	*fslib.FsLib
	perm np.Tperm
	mode np.Tmode
}

func MakeLeaderClnt(fsl *fslib.FsLib, path string, perm np.Tperm) *LeaderClnt {
	l := &LeaderClnt{}
	l.path = path
	l.FsLib = fsl
	l.perm = perm
	return l
}

func (l *LeaderClnt) AcquireLeadership(b []byte) error {
	wrt, err := l.CreateWriter(l.path, l.perm|np.DMTMP, np.OWRITE|np.OWATCH)
	if err != nil {
		db.DLPrintf("LEADER_ERR", "Create %v err %v", l.path, err)
		return err
	}

	_, err = wrt.Write(b)
	if err != nil {
		db.DLPrintf("LEADER_ERR", "Write %v err %v", l.path, err)
		return err
	}
	wrt.Close()
	return nil
}

func (l *LeaderClnt) ReleaseLeadership() error {
	err := l.Remove(l.path)
	if err != nil {
		db.DLPrintf("LEADER_ERR", "Remove %v err %v", l.path, err)
		return err
	}
	return nil
}
