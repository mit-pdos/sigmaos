package test

import (
	"testing"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// Realms commonly used for testing
const (
	REALM1 sp.Trealm = "testrealm1"
	REALM2 sp.Trealm = "testrealm2"
)

type MultiRealmTstate struct {
	T      *testing.T
	root   *Tstate
	realms map[sp.Trealm]*RealmTstate
}

func NewMultiRealmTstate(t *testing.T, realms []sp.Trealm) (*MultiRealmTstate, error) {
	mrts := &MultiRealmTstate{
		T:      t,
		root:   nil,
		realms: make(map[sp.Trealm]*RealmTstate),
	}
	rootts, err := newTstateWithRealms(t)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Err newTstateWithRealms: %v", err)
		return mrts, err
	}
	mrts.root = rootts
	for _, r := range realms {
		if err := mrts.AddRealm(r); err != nil {
			db.DPrintf(db.ALWAYS, "Err AddRealm[%v]: %v", r, err)
			return mrts, err
		}
	}
	return mrts, nil
}

func (mrts *MultiRealmTstate) AddRealmClnt(r sp.Trealm) error {
	ts, err := newRealmTstateJustClnt(mrts.root, r)
	if err != nil {
		return err
	}
	mrts.realms[r] = ts
	return nil
}

func (mrts *MultiRealmTstate) AddRealm(r sp.Trealm) error {
	ts, err := newRealmTstate(mrts.root, r)
	if err != nil {
		return err
	}
	mrts.realms[r] = ts
	return nil
}

func (mrts *MultiRealmTstate) DelRealm(r sp.Trealm, removeNamedState bool) error {
	ts := mrts.realms[r]
	err := ts.remove(removeNamedState)
	delete(mrts.realms, r)
	return err
}

func (mrts *MultiRealmTstate) AddRealmNumSubsystems(r sp.Trealm, numS3 int64, numUX int64) error {
	ts, err := newRealmTstateNumSubsystems(mrts.root, r, numS3, numUX)
	if err != nil {
		return err
	}
	mrts.realms[r] = ts
	return nil
}

func (mrts *MultiRealmTstate) GetRoot() *Tstate {
	return mrts.root
}

func (mrts *MultiRealmTstate) GetRealm(r sp.Trealm) *RealmTstate {
	return mrts.realms[r]
}

func (mrts *MultiRealmTstate) ShutdownForReboot() {
	mrts.shutdown(true)
}

func (mrts *MultiRealmTstate) Shutdown() {
	mrts.shutdown(false)
}

func (mrts *MultiRealmTstate) shutdown(willReboot bool) {
	for r := range mrts.realms {
		db.DPrintf(db.TEST, "Shut down realm %v namedState %v", r, !willReboot)
		if err := mrts.DelRealm(r, !willReboot); err != nil {
			db.DPrintf(db.ERROR, "Err remove realm[%v]: %v", r, err)
		}
	}
	if mrts.root != nil {
		mrts.root.Shutdown()
	}
}
