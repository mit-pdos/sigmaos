package realmclnt

import (
	"path"
	"sync"

	"sigmaos/fslib"
	"sigmaos/protdevclnt"
	sp "sigmaos/sigmap"
)

type RealmClnt struct {
	mu sync.Mutex
	*fslib.FsLib
	pdc *protdevclnt.ProtDevClnt
}

func MakeRealmClnt(fsl *fslib.FsLib) *RealmClnt {
	rm := &RealmClnt{FsLib: fsl}
	return rm
}

func (rc *RealmClnt) lookupRealmSrv() (string, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.pdc == nil {
		pdc, err := protdevclnt.MkProtDevClnt(rc.FsLib, sp.REALMD)
		if err != nil {
			return "", err
		}
		rc.pdc = pdc
	}
	return "", nil
}

func (rc *RealmClnt) LookupNamed(realm sp.Trealm) (string, error) {
	pn := path.Join(sp.REALMS, string(realm))
	_, err := rc.GetDir(pn)
	if err == nil {
		return pn, nil
	}
	_, err = rc.lookupRealmSrv()
	if err != nil {
		return "", err
	}
	// contact realmsrv
	return "", nil
}
