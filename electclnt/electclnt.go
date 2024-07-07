package electclnt

import (
	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/fslib"
	"sigmaos/leaderetcd"
	sp "sigmaos/sigmap"
)

//
// Library to acquire leadership
//

type ElectClnt struct {
	*fslib.FsLib
	pn    string // pathname for the leader-election file (and prefix of key)
	perm  sp.Tperm
	mode  sp.Tmode
	elect *leaderetcd.Election
	sess  *fsetcd.Session
}

func NewElectClnt(fsl *fslib.FsLib, pn string, perm sp.Tperm) (*ElectClnt, error) {
	e := &ElectClnt{FsLib: fsl, pn: pn, perm: perm}
	fs, err := fsetcd.NewFsEtcd(fsl.GetNetProxyClnt().Dial, fsl.ProcEnv().GetEtcdEndpoints(), fsl.ProcEnv().GetRealm(), nil)
	if err != nil {
		return nil, err
	}
	e.sess, err = fs.NewSession()
	if err != nil {
		return nil, err
	}
	e.elect, err = leaderetcd.NewElection(e.ProcEnv(), e.sess, pn)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (ec *ElectClnt) AcquireLeadership(b []byte) error {
	if err := ec.elect.Candidate(); err != nil {
		return err
	}
	pn := ec.elect.Key()
	db.DPrintf(db.ELECTCLNT, "CreateLeaderFile %v lid %v f %v\n", pn, ec.sess.Lease(), ec.Fence())
	if err := ec.CreateLeaderFile(pn, b, ec.sess.Lease(), ec.Fence()); err != nil {
		return err
	}
	return nil
}

func (ec *ElectClnt) ReleaseLeadership() error {
	ec.Remove(ec.elect.Key())
	return ec.elect.Resign()
}

func (ec *ElectClnt) Fence() sp.Tfence {
	return ec.elect.Fence()
}

func (ec *ElectClnt) Lease() sp.TleaseId {
	return ec.sess.Lease()
}
