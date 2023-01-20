package realmclnt

import (
	"sync"

	"sigmaos/fslib"
	"sigmaos/protdevclnt"
	"sigmaos/realmsrv/proto"
	sp "sigmaos/sigmap"
)

type RealmClnt struct {
	mu sync.Mutex
	*fslib.FsLib
	pdc    *protdevclnt.ProtDevClnt
	nameds map[sp.Trealm][]string
}

func MakeRealmClnt(fsl *fslib.FsLib) *RealmClnt {
	rm := &RealmClnt{FsLib: fsl}
	rm.nameds = make(map[sp.Trealm][]string)
	return rm
}

func (rc *RealmClnt) lookupRealmSrv() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.pdc == nil {
		pdc, err := protdevclnt.MkProtDevClnt(rc.FsLib, sp.REALMD)
		if err != nil {
			return err
		}
		rc.pdc = pdc
	}
	return nil
}

func (rc *RealmClnt) lookupNamed(realm sp.Trealm) ([]string, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	nds, ok := rc.nameds[realm]
	return nds, ok
}

func (rc *RealmClnt) LookupNamed(realm sp.Trealm) ([]string, error) {
	nds, ok := rc.lookupNamed(realm)
	if ok {
		return nds, nil
	}
	err := rc.lookupRealmSrv()
	if err != nil {
		return nil, err
	}
	req := &proto.MakeRequest{
		Realm: string(realm),
	}
	res := &proto.MakeResult{}
	if err := rc.pdc.RPC("RealmSrv.Make", req, res); err != nil {
		return nil, err
	}
	return res.NamedAddr, nil
}
