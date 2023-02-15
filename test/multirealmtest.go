package test

import (
	"testing"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Tstate relative to a realm.
type RealmTstate struct {
	realm sp.Trealm
	*sigmaclnt.SigmaClnt
	T *testing.T
}

// Creates a realm, and a tstate relative to that realm.
func MakeRealmTstate(ts *Tstate, realm sp.Trealm) *RealmTstate {
	var err error
	net := ""
	if Overlays {
		net = realm.String()
	}
	if err = ts.rc.MakeRealm(realm, net); err != nil {
		db.DFatalf("Error MakeRealmTstate MkRealm: %v", err)
	}
	var sc *sigmaclnt.SigmaClnt
	if sc, err = sigmaclnt.MkSigmaClntRealm(ts.FsLib, "test", realm); err != nil {
		db.DFatalf("Error MakeRealmTstate MkSigmaClnt: %v", err)
	}
	return &RealmTstate{
		realm:     realm,
		SigmaClnt: sc,
		T:         ts.T,
	}
}

func (rts *RealmTstate) GetRealm() sp.Trealm {
	return rts.realm
}
