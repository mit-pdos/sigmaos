package test

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/realmsrv"
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
	return newRealmTstateClnt(ts, realm, true, realmsrv.SUBSYSTEM_PER_NODE, realmsrv.SUBSYSTEM_PER_NODE)
}

func NewRealmTstateNumSubsystems(ts *Tstate, realm sp.Trealm, numS3 int64, numUX int64) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true, numS3, numUX)
}

// Creates a tstate relative to an existing realm.
func NewRealmTstateClnt(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, false, realmsrv.SUBSYSTEM_PER_NODE, realmsrv.SUBSYSTEM_PER_NODE)
}

func newRealmTstateClnt(ts *Tstate, realm sp.Trealm, newrealm bool, numS3 int64, numUX int64) (*RealmTstate, error) {
	if newrealm {
		net := ""
		db.DPrintf(db.TEST, "Make realm %v", realm)
		if err := ts.rc.NewRealm(realm, net, numS3, numUX); err != nil {
			db.DPrintf(db.ERROR, "Error NewRealmTstate NewRealm: %v", err)
			return nil, err
		}
		db.DPrintf(db.TEST, "Done making realm %v", realm)
	}
	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), realm)
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
