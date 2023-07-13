package leaderclnt

import (
	"hash/fnv"

	"sigmaos/electclnt"
	"sigmaos/fenceclnt"
	"sigmaos/fslib"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

//
// Library for becoming a leader for an epoch.
//

type LeaderClnt struct {
	*fslib.FsLib
	*fenceclnt.FenceClnt
	e     *electclnt.ElectClnt
	fence *sessp.Tfence
	pn    string
}

func MakeLeaderClnt(fsl *fslib.FsLib, pn string, perm sp.Tperm) (*LeaderClnt, error) {
	l := &LeaderClnt{FsLib: fsl, pn: pn, FenceClnt: fenceclnt.MakeFenceClnt(fsl)}
	e, err := electclnt.MakeElectClnt(fsl, pn, perm)
	if err != nil {
		return nil, err
	}
	l.e = e
	return l, nil
}

// Become leader and fence ops at that epoch.  Another proc may steal
// our leadership (e.g., after we are partioned) and start a higher
// epoch.  Note epoch doesn't take effect until we perform a fenced
// operation (e.g., a read/write).
func (l *LeaderClnt) LeadAndFence(b []byte, dirs []string) error {
	if err := l.e.AcquireLeadership(b); err != nil {
		return err
	}
	l.fence = sessp.MakeFenceNull()
	l.fence.Epoch = uint64(l.e.Epoch())
	h := fnv.New64a()
	h.Write([]byte(l.pn))
	l.fence.Fenceid.Path = h.Sum64()
	return l.fenceDirs(dirs)
}

// Enter next epoch.  If the leader is partitioned and another leader
// has taken over already, this fails.
func (l *LeaderClnt) fenceDirs(dirs []string) error {
	if err := l.FenceAtEpoch(l.fence, dirs); err != nil {
		return err
	}
	return nil
}

func (l *LeaderClnt) ReleaseLeadership() error {
	return l.e.ReleaseLeadership()
}
