package realmsrv

import (
	"os"
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	"sigmaos/realmsrv/proto"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	MIN_PORT = 30000
)

type RealmSrv struct {
	mu         sync.Mutex
	sc         *sigmaclnt.SigmaClnt
	lastNDPort int
	ch         chan struct{}
}

func RunRealmSrv() error {
	rs := &RealmSrv{
		lastNDPort: MIN_PORT,
	}
	rs.ch = make(chan struct{})
	db.DPrintf(db.REALMD, "%v: Run %v %s\n", proc.GetName(), sp.REALMD, os.Environ())
	pds, err := protdevsrv.MakeProtDevSrv(sp.REALMD, rs)
	if err != nil {
		return err
	}
	_, serr := pds.MemFs.Create(sp.REALMSREL, 0777|sp.DMDIR, sp.OREAD)
	if serr != nil {
		return serr
	}
	db.DPrintf(db.REALMD, "%v: makesrv ok\n", proc.GetName())
	rs.sc = pds.MemFs.SigmaClnt()
	err = pds.RunServer()
	return nil
}

func (rm *RealmSrv) Make(req proto.MakeRequest, res *proto.MakeResult) error {
	db.DPrintf(db.REALMD, "RealmSrv.Make %v\n", req.Realm)
	rid := sp.Trealm(req.Realm)
	pn := path.Join(sp.REALMS, req.Realm)
	p := proc.MakeProc("named", []string{":0", req.Realm, pn})
	if _, errs := rm.sc.SpawnBurst([]*proc.Proc{p}); len(errs) != 0 {
		return errs[0]
	}
	if err := rm.sc.WaitStart(p.GetPid()); err != nil {
		return err
	}

	db.DPrintf(db.REALMD, "RealmSrv.Make named for %v started\n", rid)

	sc, err := sigmaclnt.MkSigmaClntRealm(rm.sc.FsLib, "realmd", rid)
	if err != nil {
		return err
	}

	// Make some rootrealm services available in new realm
	// XXX add sp.DBREL
	for _, s := range []string{sp.SCHEDDREL, sp.UXREL, sp.S3REL} {
		pn := path.Join(sp.NAMED, s)
		mnt := sp.Tmount{Addr: rm.sc.NamedAddr(), Root: s}
		db.DPrintf(db.REALMD, "Link %v at %s\n", mnt, pn)
		if err := sc.MountService(pn, mnt); err != nil {
			db.DPrintf(db.REALMD, "MountService %v err %v\n", pn, err)
			return err
		}
	}

	// Make some realm dirs
	for _, s := range []string{sp.KPIDSREL} {
		pn := path.Join(sp.NAMED, s)
		db.DPrintf(db.REALMD, "Mkdir %v", pn)
		if err := sc.MkDir(pn, 0777); err != nil {
			db.DPrintf(db.REALMD, "MountService %v err %v\n", pn, err)
			return err
		}
	}
	return nil
}
