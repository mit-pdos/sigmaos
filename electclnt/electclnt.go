package electclnt

import (
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	"sigmaos/leaderetcd"
	"sigmaos/proc"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

//
// Library to acquire leadership
//

type ElectClnt struct {
	*fslib.FsLib
	pn    string // pathname for the leader-election file
	perm  sp.Tperm
	mode  sp.Tmode
	elect *leaderetcd.Election
	sess  *fsetcd.Session
}

func MakeElectClnt(fsl *fslib.FsLib, pn string, perm sp.Tperm) (*ElectClnt, error) {
	e := &ElectClnt{FsLib: fsl, pn: pn, perm: perm}
	fs, err := fsetcd.MkFsEtcd(proc.GetRealm())
	if err != nil {
		return nil, err
	}
	e.sess, err = fs.NewSession()
	if err != nil {
		return nil, err
	}
	e.elect, err = leaderetcd.MkElection(e.sess, pn)
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
	db.DPrintf(db.ELECTCLNT, "CreateLeaderFile %v lid %v\n", pn, ec.sess.Lease())
	if err := ec.CreateLeaderFile(pn, b, ec.sess.Lease()); err != nil {
		return err
	}
	return nil
}

func (ec *ElectClnt) ReleaseLeadership() error {
	ec.Remove(ec.elect.Key())
	return ec.elect.Resign()
}

func (ec *ElectClnt) Epoch() sessp.Tepoch {
	return sessp.Tepoch(ec.elect.Rev())
}

func (ec *ElectClnt) Lease() sp.TleaseId {
	return ec.sess.Lease()
}
