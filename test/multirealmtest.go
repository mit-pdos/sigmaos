package test

import (
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
func NewRealmTstate(ts *Tstate, realm sp.Trealm) *RealmTstate {
	return newRealmTstateClnt(ts, realm, true)
}

// News a tstate relative to an existing realm.
func NewRealmTstateClnt(ts *Tstate, realm sp.Trealm) *RealmTstate {
	return newRealmTstateClnt(ts, realm, false)
}

func newRealmTstateClnt(ts *Tstate, realm sp.Trealm, newrealm bool) *RealmTstate {
	if newrealm {
		net := ""
		if Overlays {
			net = realm.String()
		}
		db.DPrintf(db.TEST, "Make realm %v", realm)
		if err := ts.rc.NewRealm(realm, net); err != nil {
			db.DFatalf("Error NewRealmTstate NewRealm: %v", err)
		}
		db.DPrintf(db.TEST, "Done making realm %v", realm)
	}
	pcfg := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), realm)
	db.DPrintf(db.TEST, "ProcEnv for new realm %v", pcfg)
	if sc, err := sigmaclnt.NewSigmaClntRootInit(pcfg); err != nil {
		db.DFatalf("Error NewRealmTstate NewSigmaClnt: %v", err)
	} else {
		return &RealmTstate{
			realm:     realm,
			SigmaClnt: sc,
			Ts:        ts,
		}
	}
	return nil
}

func (rts *RealmTstate) GetRealm() sp.Trealm {
	return rts.realm
}

func (rts *RealmTstate) Remove() error {
	return rts.Ts.rc.RemoveRealm(rts.realm)
}
