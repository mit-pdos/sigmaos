package test

import (
	"path/filepath"
	"time"

	"github.com/stretchr/testify/assert"

	bootclnt "sigmaos/boot/clnt"
	db "sigmaos/debug"
	kernelclnt "sigmaos/kernel/clnt"
	"sigmaos/path"
	"sigmaos/proc"
	realmpkg "sigmaos/realm"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
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
func newRealmTstate(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true, realmpkg.SUBSYSTEM_PER_NODE, realmpkg.SUBSYSTEM_PER_NODE)
}

func newRealmTstateNumSubsystems(ts *Tstate, realm sp.Trealm, numS3 int64, numUX int64) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true, numS3, numUX)
}

// Creates a tstate relative to an existing realm.
func newRealmTstateJustClnt(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
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

func (rts *RealmTstate) remove(removeNamedState bool) error {
	for i, pid := range rts.pkss {
		if err := rts.mkc.EvictKernelProc(rts.pksskids[i], pid); err != nil {
			db.DPrintf(db.ALWAYS, "Error evict kernel proc %v kid %v", pid, rts.pksskids[i])
		}
	}
	rts.mkc.StopWatching()
	return rts.Ts.rc.RemoveRealm(rts.realm, removeNamedState)
}

func (rts *RealmTstate) BootNode(n int) error {
	return rts.bootNode(n, false)
}

// We may need to wait for the realm's new named to come up during
// booting the node so that the realm's UX and S3 can register
// themselves.
func (rts *RealmTstate) bootNode(n int, waitForNamed bool) error {
	kids, err := rts.Ts.bootNode(n, bootclnt.BOOT_NODE)
	if err != nil {
		return err
	}
	db.DPrintf(db.TEST, "Booted additional kernels: %v", kids)
	if waitForNamed {
		db.DPrintf(db.TEST, "Wait for realm %v named to come up", rts.realm)
		// Indicate that the link with named must be resolved, in case
		// the endpoint for the old named is cached; a failed
		// connection will invalidate the endpoint.
		pn := path.MarkResolve(filepath.Join(sp.REALMS, rts.realm.String()))

		// Loop until new named is up.  Don't use GetFileWatch because
		// it may return the old named's EP, since it persists until
		// the new one overrides it.
		for i := 0; true; i++ {
			if sts, err := rts.Ts.GetDir(pn); err != nil {
				db.DPrintf(db.TEST, "Named down %v pn %v err %v", rts.realm, pn, err)
				if i >= sp.Conf.Path.MAX_RESOLVE_RETRY {
					return err
				}
				time.Sleep(sp.Conf.Path.RESOLVE_TIMEOUT)
			} else {
				db.DPrintf(db.TEST, "Done wait for realm named %v", sp.Names(sts))
				break
			}
		}

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

func (rts *RealmTstate) CrashServer(e0, e1 crash.Tevent, srv string) {
	db.DPrintf(db.ALWAYS, "Crash %v srv %v realm", e0.Path, srv)
	err := crash.SignalFailer(rts.Ts.FsLib, e0.Path)
	if !assert.Nil(rts.Ts.T, err) {
		db.DPrintf(db.TEST, "SignalFailer %v err %v", e0.Path, err)
	}
	em := crash.NewTeventMapOne(e1)
	s, err := em.Events2String()
	assert.Nil(rts.Ts.T, err)
	switch srv {
	case sp.MSCHEDREL, sp.PROCDREL, sp.UXREL:
		if srv == sp.MSCHEDREL || srv == sp.PROCDREL {
			// Wait for old named potentially to exit and its lease to expire
			db.DPrintf(db.TEST, "Waiting for named's lease to (potentially) expire")
			time.Sleep(2*sp.Conf.Session.TIMEOUT + sp.EtcdSessionExpired*time.Second)
			db.DPrintf(db.TEST, "Done waiting for named's lease to (potentially) expire")
		}
		// a crashed msched and procd causes several kernel services
		// to exit, so start a new node. if the crashed msched started
		// named, the named will crash too (because its waitExit() to
		// msched will timeout). but realmd will start a new one (for
		// which we may have to wait until it has started).
		err = rts.bootNode(1, true)
	default:
		err = rts.Ts.BootEnv(srv, []string{"SIGMAFAIL=" + s})
	}
	assert.Nil(rts.Ts.T, err, "Error Boot: %v", err)
	db.DPrintf(db.ALWAYS, "Booted %v %v", e1.Path, em)
}
