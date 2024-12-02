// Package leaderclnt allows a proc to become a leader for an epoch
// and fence its operations so that its operations will fail in
// subsequent epochs.
package leaderclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leaderclnt/electclnt"
	"sigmaos/leaderclnt/fenceclnt"
	sp "sigmaos/sigmap"
)

type LeaderClnt struct {
	fc *fenceclnt.FenceClnt
	ec *electclnt.ElectClnt
	pn string
}

func NewLeaderClnt(fsl *fslib.FsLib, pn string, perm sp.Tperm) (*LeaderClnt, error) {
	l := &LeaderClnt{pn: pn, fc: fenceclnt.NewFenceClnt(fsl)}
	ec, err := electclnt.NewElectClnt(fsl, pn, perm)
	if err != nil {
		return nil, err
	}
	l.ec = ec
	return l, nil
}

// Become leader and fence ops at that epoch.  Another proc may steal
// our leadership (e.g., after we are partioned) and start a higher
// epoch.  Note epoch doesn't take effect until we perform a fenced
// operation (e.g., a read/write).
func (l *LeaderClnt) LeadAndFence(b []byte, dirs []string) error {
	if err := l.ec.AcquireLeadership(b); err != nil {
		return err
	}
	db.DPrintf(db.LEADER, "LeadAndFence: %v\n", l.Fence())
	return l.fenceDirs(dirs)
}

func (l *LeaderClnt) Fence() sp.Tfence {
	return l.ec.Fence()
}

func (l *LeaderClnt) fenceDirs(dirs []string) error {
	if err := l.fc.FenceAtEpoch(l.Fence(), dirs); err != nil {
		return err
	}
	return nil
}

func (l *LeaderClnt) GetFences(pn string) ([]*sp.Tstat, error) {
	return l.fc.GetFences(pn)
}

// Works for file systems that support fencefs
func (l *LeaderClnt) RemoveFence(dirs []string) error {
	return l.fc.RemoveFence(dirs)
}

func (l *LeaderClnt) ReleaseLeadership() error {
	return l.ec.ReleaseLeadership()
}

func (l *LeaderClnt) Lease() sp.TleaseId {
	return l.ec.Lease()
}
