package test

import (
	bootclnt "sigmaos/boot/clnt"
	db "sigmaos/debug"
	kernelclnt "sigmaos/kernel/clnt"
	"sigmaos/proc"
	realmpkg "sigmaos/realm"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Tstate relative to a realm.
type RealmTstate struct {
	realm sp.Trealm
	*sigmaclnt.SigmaClnt
	Ts       *Tstate
	mkc      *kernelclnt.MultiKernelClnt
	pkss     []sp.Tpid
	pksskids []string
}

// Creates a realm, and a tstate relative to that realm.
func NewRealmTstate(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true, realmpkg.SUBSYSTEM_PER_NODE, realmpkg.SUBSYSTEM_PER_NODE)
}

func NewRealmTstateNumSubsystems(ts *Tstate, realm sp.Trealm, numS3 int64, numUX int64) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true, numS3, numUX)
}

// Creates a tstate relative to an existing realm.
func NewRealmTstateClnt(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, false, realmpkg.SUBSYSTEM_PER_NODE, realmpkg.SUBSYSTEM_PER_NODE)
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
			pkss:      []sp.Tpid{},
			pksskids:  []string{},
			mkc:       kernelclnt.NewMultiKernelClnt(ts.FsLib, db.NEVER, db.TEST),
		}, nil
	}
}

func (rts *RealmTstate) GetRealm() sp.Trealm {
	return rts.realm
}

func (rts *RealmTstate) remove() error {
	for i, pid := range rts.pkss {
		if err := rts.mkc.EvictKernelProc(rts.pksskids[i], pid); err != nil {
			db.DPrintf(db.ALWAYS, "Error evict kernel proc %v kid %v", pid, rts.pksskids[i])
		}
	}
	rts.mkc.StopWatching()
	return rts.Ts.rc.RemoveRealm(rts.realm)
}

func (rts *RealmTstate) BootNode(n int) error {
	kids, err := rts.Ts.bootNode(n, bootclnt.BOOT_NODE)
	if err != nil {
		return err
	}
	for _, kid := range kids {
		for _, ss := range []string{sp.UXREL, sp.S3REL} {
			pid, err := rts.mkc.BootInRealm(kid, rts.realm, ss, nil)
			if err != nil {
				db.DPrintf(db.ALWAYS, "Error boot %v in realm %v on kid %v", ss, rts.realm, kid)
				return err
			}
			rts.pkss = append(rts.pkss, pid)
			rts.pksskids = append(rts.pksskids, kid)
		}
	}
	db.DPrintf(db.BOOT, "Boot additional kernel subsystems for realm %v on kids %v", rts.realm, kids)
	return nil
}
