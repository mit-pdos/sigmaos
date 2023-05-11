package realmclnt

import (
	"sigmaos/fslib"
	"sigmaos/protdevclnt"
	"sigmaos/realmsrv/proto"
	sp "sigmaos/sigmap"
)

type RealmClnt struct {
	*fslib.FsLib
	pdc *protdevclnt.ProtDevClnt
}

func MakeRealmClnt(fsl *fslib.FsLib) (*RealmClnt, error) {
	rc := &RealmClnt{FsLib: fsl}
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{rc.FsLib}, sp.REALMD)
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
