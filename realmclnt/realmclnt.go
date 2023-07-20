package realmclnt

import (
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/realmsrv/proto"
	sp "sigmaos/sigmap"
)

type RealmClnt struct {
	*fslib.FsLib
	pdc *rpcclnt.RPCClnt
}

func MakeRealmClnt(fsl *fslib.FsLib) (*RealmClnt, error) {
	rc := &RealmClnt{FsLib: fsl}
	pdc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{rc.FsLib}, sp.REALMD)
	if err != nil {
		return nil, err
	}
	rc.pdc = pdc
	return rc, nil
}

func (rc *RealmClnt) MakeRealm(realm sp.Trealm, net string) error {
	req := &proto.MakeRequest{
		Realm:   realm.String(),
		Network: net,
	}
	res := &proto.MakeResult{}
	if err := rc.pdc.RPC("RealmSrv.Make", req, res); err != nil {
		return err
	}
	return nil
}

func (rc *RealmClnt) RemoveRealm(realm sp.Trealm) error {
	req := &proto.RemoveRequest{
		Realm: realm.String(),
	}
	res := &proto.RemoveResult{}
	if err := rc.pdc.RPC("RealmSrv.Remove", req, res); err != nil {
		return err
	}
	return nil
}
