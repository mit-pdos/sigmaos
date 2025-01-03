package namesrv

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/namesrv/leaderetcd"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// XXX maybe in fsetd
func Elect(fs *fsetcd.FsEtcd, pe *proc.ProcEnv, realm sp.Trealm) (*fsetcd.Session, *leaderetcd.Election, error) {
	fn := fmt.Sprintf("named-election-%s", realm)
	sess, err := fs.NewSession()
	if err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.NAMED, "created fsetcd session")
	elect, err := leaderetcd.NewElection(pe, sess, fn)
	if err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.NAMED, "started leaderetcd session")
	if err := elect.Candidate(); err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.NAMED, "succeeded leaderetcd election")
	return sess, elect, nil
}

func (nd *Named) startLeader() error {
	nd.pstats = fsetcd.NewPstatsDev()
	fs, err := fsetcd.NewFsEtcd(nd.GetDialProxyClnt().Dial, nd.ProcEnv().GetEtcdEndpoints(), nd.realm, nd.pstats)
	if err != nil {
		return err
	}
	nd.fs = fs
	db.DPrintf(db.NAMED, "created fsetcd client")

	nd.sess, nd.elect, err = Elect(fs, nd.ProcEnv(), nd.realm)
	if err != nil {
		return err
	}

	if err := nd.fs.WatchLeased(nd.ephch); err != nil {
		return err
	}

	go nd.watchLeased()

	fs.Fence(nd.elect.Key(), nd.elect.Rev())

	db.DPrintf(db.NAMED, "leader %v %v\n", nd.realm, nd.elect.Key())

	return nil
}
