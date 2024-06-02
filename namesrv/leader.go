package namesrv

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/leaderetcd"
)

func (nd *Named) startLeader() error {
	fs, err := fsetcd.NewFsEtcd(nd.GetNetProxyClnt().Dial, nd.ProcEnv().GetEtcdEndpoints(), nd.realm)
	if err != nil {
		return err
	}
	nd.fs = fs
	fn := fmt.Sprintf("named-election-%s", nd.realm)
	db.DPrintf(db.NAMED, "created fsetcd client")

	sess, err := fs.NewSession()
	if err != nil {
		return err
	}
	nd.sess = sess

	db.DPrintf(db.NAMED, "created fsetcd session")

	nd.elect, err = leaderetcd.NewElection(nd.ProcEnv(), nd.sess, fn)
	if err != nil {
		return err
	}
	db.DPrintf(db.NAMED, "started leaderetcd session")

	if err := nd.elect.Candidate(); err != nil {
		return err
	}

	db.DPrintf(db.NAMED, "succeeded leaderetcd election")

	if err := nd.fs.WatchEphemeral(nd.ephch); err != nil {
		return err
	}

	go nd.watchEphemeral()

	fs.Fence(nd.elect.Key(), nd.elect.Rev())

	db.DPrintf(db.NAMED, "leader %v %v\n", nd.realm, nd.elect.Key())

	return nil
}
