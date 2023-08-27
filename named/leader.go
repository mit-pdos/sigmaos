package named

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/leaderetcd"
)

func (nd *Named) startLeader() error {
	fs, err := fsetcd.MkFsEtcd(nd.SigmaConfig())
	if err != nil {
		return err
	}
	nd.fs = fs
	fn := fmt.Sprintf("named-election-%s", nd.realm)

	sess, err := fs.NewSession()
	if err != nil {
		return err
	}
	nd.sess = sess

	nd.elect, err = leaderetcd.MkElection(nd.SigmaConfig(), nd.sess, fn)
	if err != nil {
		return err
	}

	if err := nd.elect.Candidate(); err != nil {
		return err
	}

	fs.Fence(nd.elect.Key(), nd.elect.Rev())

	db.DPrintf(db.NAMED, "leader %v %v\n", nd.realm, nd.elect.Key())

	return nil
}
