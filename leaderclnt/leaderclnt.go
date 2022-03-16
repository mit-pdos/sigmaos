package leaderclnt

import (
	"ulambda/electclnt"
	"ulambda/epochclnt"
	"ulambda/fenceclnt1"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type LeaderClnt struct {
	*fslib.FsLib
	epochfn string
	e       *electclnt.ElectClnt
	ec      *epochclnt.EpochClnt
	fc      *fenceclnt1.FenceClnt
}

func MakeLeaderClnt(fsl *fslib.FsLib, leaderfn string, perm np.Tperm, dirs []string) *LeaderClnt {
	l := &LeaderClnt{}
	l.FsLib = fsl
	l.epochfn = leaderfn + "-epoch"
	l.e = electclnt.MakeElectClnt(fsl, leaderfn, 0)
	l.ec = epochclnt.MakeEpochClnt(fsl, l.epochfn, perm)
	l.fc = fenceclnt1.MakeFenceClnt(fsl, l.ec, perm, dirs)
	return l
}

func (l *LeaderClnt) EpochPath() string {
	return l.epochfn
}

// Become leader for an epoch and fence op for that epoch.  Another
// proc may steal our leadership (e.g., after we are partioned) and
// start a higher epoch.  Note epoch doesn't take effect until we
// perform a fenced operation (e.g., a read/write).
func (l *LeaderClnt) AcquireFencedEpoch() (np.Tepoch, error) {
	if err := l.e.AcquireLeadership(); err != nil {
		return np.NoEpoch, err
	}
	epoch, err := l.ec.AdvanceEpoch()
	if err != nil {
		return np.NoEpoch, err
	}
	if err := l.fc.FenceAtEpoch(epoch); err != nil {
		return np.NoEpoch, err
	}
	return epoch, nil
}
