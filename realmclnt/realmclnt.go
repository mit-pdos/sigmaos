package realmclnt

import (
	"sigmaos/fslib"
	"sigmaos/realmsrv/proto"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
)

type RealmClnt struct {
	*fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewRealmClnt(fsl *fslib.FsLib) (*RealmClnt, error) {
	rc := &RealmClnt{FsLib: fsl}
	ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{rc.FsLib}, sp.REALMD)
	if err != nil {
		return nil, err
	}
	rpcc := rpcclnt.NewRPCClnt(ch)
	rc.rpcc = rpcc
	return rc, nil
}

func (rc *RealmClnt) NewRealm(realm sp.Trealm, net string, numS3 int64, numUX int64) error {
	req := &proto.MakeRequest{
		Realm:   realm.String(),
		NumS3:   numS3,
		NumUX:   numUX,
		Network: net,
	}
	res := &proto.MakeResult{}
	if err := rc.rpcc.RPC("RealmSrv.Make", req, res); err != nil {
		return err
	}
	return nil
}

func (rc *RealmClnt) RemoveRealm(realm sp.Trealm) error {
	req := &proto.RemoveRequest{
		Realm: realm.String(),
	}
	res := &proto.RemoveResult{}
	if err := rc.rpcc.RPC("RealmSrv.Remove", req, res); err != nil {
		return err
	}
	return nil
}
