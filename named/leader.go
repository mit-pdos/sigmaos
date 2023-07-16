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
	fs, err := fsetcd.MkFsEtcd(nd.realm)
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

	nd.elect, err = leaderetcd.MkElection(nd.sess, fn)
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

	fs.Fence(nd.elect.Key(), nd.elect.Rev())

	root := rootDir(fs, nd.realm)
	srv := fslibsrv.BootSrv(root, ip+":0", nd.SigmaClnt, nd.attach, nd.detach, nil)
	if srv == nil {
		return fmt.Errorf("BootSrv err %v\n", err)
	}
	nd.SessSrv = srv

	db.DPrintf(db.NAMED, "leader %v %v %v\n", nd.realm, srv.MyAddr(), nd.elect.Key())
	return nil
}