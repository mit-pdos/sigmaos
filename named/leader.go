package named

import (
	"fmt"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslibsrv"
	"sigmaos/leaderetcd"
)

func (nd *Named) startLeader() error {
	ec, err := fsetcd.MkEtcdClnt(nd.realm)
	if err != nil {
		return err
	}
	nd.ec = ec
	fn := fmt.Sprintf("named-election-%s", nd.realm)

	nd.elect, err = leaderetcd.MkElection(nd.ec.Client, fn)
	if err != nil {
		return err
	}

	if err := nd.elect.Candidate(); err != nil {
		return err
	}

	ip, err := container.LocalIP()
	if err != nil {
		return err
	}

	ec.Fence(nd.elect.Key(), nd.elect.Rev())

	root := rootDir(ec, nd.realm)
	srv := fslibsrv.BootSrv(root, ip+":0", "named", nd.SigmaClnt)
	if srv == nil {
		db.DFatalf("MakeReplServer err %v", err)
	}
	nd.SessSrv = srv

	db.DPrintf(db.NAMED, "leader %v %v\n", nd.realm, srv.MyAddr())
	return nil
}
