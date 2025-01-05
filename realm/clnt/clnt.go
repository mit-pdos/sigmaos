package clnt

import (
	"sigmaos/realm/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type RealmClnt struct {
	*fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewRealmClnt(fsl *fslib.FsLib) (*RealmClnt, error) {
	rc := &RealmClnt{FsLib: fsl}
	rpcc, err := sprpcclnt.NewRPCClnt(rc.FsLib, sp.REALMD)
	if err != nil {
		return nil, err
	}
	rc.rpcc = rpcc
	return rc, nil
}

func (rc *RealmClnt) NewRealm(realm sp.Trealm, net string, numS3 int64, numUX int64) error {
	req := &proto.MakeReq{
		Realm:   realm.String(),
		NumS3:   numS3,
		NumUX:   numUX,
		Network: net,
	}
	res := &proto.MakeRep{}
	if err := rc.rpcc.RPC("RealmSrv.Make", req, res); err != nil {
		return err
	}
	return nil
}

func (rc *RealmClnt) RemoveRealm(realm sp.Trealm) error {
	req := &proto.RemoveReq{
		Realm: realm.String(),
	}
	res := &proto.RemoveRep{}
	if err := rc.rpcc.RPC("RealmSrv.Remove", req, res); err != nil {
		return err
	}
	return nil
}
