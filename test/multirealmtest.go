package test

import (
	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Tstate relative to a realm.
type RealmTstate struct {
	realm sp.Trealm
	*sigmaclnt.SigmaClnt
	Ts *Tstate
}

// Creates a realm, and a tstate relative to that realm.
func NewRealmTstate(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true)
}

// Creates a tstate relative to an existing realm.
func NewRealmTstateClnt(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, false)
}

func newRealmTstateClnt(ts *Tstate, realm sp.Trealm, newrealm bool) (*RealmTstate, error) {
	if newrealm {
		net := ""
		if Overlays {
			net = realm.String()
		}
		db.DPrintf(db.TEST, "Make realm %v", realm)
		if err := ts.rc.NewRealm(realm, net); err != nil {
			db.DPrintf(db.ERROR, "Error NewRealmTstate NewRealm: %v", err)
			return nil, err
		}
		db.DPrintf(db.TEST, "Done making realm %v", realm)
	}
	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), realm)
	pc := auth.NewProcClaims(pe)
	pc.AllowedPaths = sp.ALL_PATHS
	token, err := ts.MintToken(pc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error MintToken: %v", err)
		return nil, err
	}
	pe.SetToken(token)
	db.DPrintf(db.TEST, "ProcEnv for new realm %v", pe)
	if sc, err := sigmaclnt.NewSigmaClntRootInit(pe); err != nil {
		db.DPrintf(db.ERROR, "Error NewRealmTstate NewSigmaClnt: %v", err)
		return nil, err
	} else {
		return &RealmTstate{
			realm:     realm,
			SigmaClnt: sc,
			Ts:        ts,
		}, nil
	}
}

func (rts *RealmTstate) GetRealm() sp.Trealm {
	return rts.realm
}

func (rts *RealmTstate) Remove() error {
	return rts.Ts.rc.RemoveRealm(rts.realm)
}
