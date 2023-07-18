package realmsrv

import (
	"os"
	"os/exec"
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/leasemgrsrv"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	"sigmaos/realmsrv/proto"
	"sigmaos/semclnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	MKNET      = "./bin/kernel/create-net.sh"
	MIN_PORT   = 30000
	NAMED_MCPU = 1000
)

type Realm struct {
	named *proc.Proc // XXX groupmgr for fault tolerance
	sc    *sigmaclnt.SigmaClnt
}

type RealmSrv struct {
	mu         sync.Mutex
	realms     map[sp.Trealm]*Realm
	sc         *sigmaclnt.SigmaClnt
	lastNDPort int
	ch         chan struct{}
}

func RunRealmSrv() error {
	rs := &RealmSrv{
		lastNDPort: MIN_PORT,
		realms:     make(map[sp.Trealm]*Realm),
	}
	rs.ch = make(chan struct{})
	db.DPrintf(db.REALMD, "%v: Run %v %s\n", proc.GetName(), sp.REALMD, os.Environ())
	pds, err := protdevsrv.MakeProtDevSrv(sp.REALMD, rs)
	if err != nil {
		return err
	}
	if err := leasemgrsrv.NewLeaseSrv(pds.MemFs); err != nil {
		return err
	}
	_, serr := pds.MemFs.Create(sp.REALMSREL, 0777|sp.DMDIR, sp.OREAD, sp.NoLeaseId)
	if serr != nil {
		return serr
	}
	db.DPrintf(db.REALMD, "%v: makesrv ok\n", proc.GetName())
	rs.sc = pds.MemFs.SigmaClnt()
	err = pds.RunServer()
	return nil
}

func MkNet(net string) error {
	if net == "" {
		return nil
	}
	args := []string{"sigmanet-" + net}
	out, err := exec.Command(MKNET, args...).Output()
	if err != nil {
		db.DPrintf(db.REALMD, "MkNet: %v %s err %v\n", net, string(out), err)
		return err
	}
	db.DPrintf(db.REALMD, "MkNet: %v\n", string(out))
	return nil
}

// XXX clean up if fail during Make
func (rm *RealmSrv) Make(ctx fs.CtxI, req proto.MakeRequest, res *proto.MakeResult) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	db.DPrintf(db.REALMD, "RealmSrv.Make %v %v\n", req.Realm, req.Network)
	rid := sp.Trealm(req.Realm)
	// If realm already exists
	if _, ok := rm.realms[rid]; ok {
		return serr.MkErr(serr.TErrExists, rid)
	}
	if err := MkNet(req.Network); err != nil {
		return err
	}

	p := proc.MakeProc("named", []string{req.Realm, "0"})
	p.SetMcpu(NAMED_MCPU)

	if _, errs := rm.sc.SpawnBurst([]*proc.Proc{p}, 2); len(errs) != 0 {
		db.DPrintf(db.REALMD_ERR, "Error SpawnBurst: %v", errs[0])
		return errs[0]
	}
	if err := rm.sc.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.REALMD_ERR, "Error WaitStart: %v", err)
		return err
	}

	// wait until realm's named is ready to serve
	sem := semclnt.MakeSemClnt(rm.sc.FsLib, path.Join(sp.REALMS, req.Realm)+".sem")
	if err := sem.Down(); err != nil {
		return err
	}

	db.DPrintf(db.REALMD, "RealmSrv.Make named for %v started\n", rid)

	sc, err := sigmaclnt.MkSigmaClntRealmFsLib(rm.sc.FsLib, "realmd", rid)
	if err != nil {
		db.DPrintf(db.REALMD_ERR, "Error MkSigmaClntRealm: %v", err)
		return err
	}
	// Make some rootrealm services available in new realm
	for _, s := range []string{sp.SCHEDDREL, sp.UXREL, sp.S3REL, sp.DBREL, sp.BOOTREL} {
		pn := path.Join(sp.NAMED, s)
		mnt := sp.Tmount{Addr: rm.sc.NamedAddr(), Root: s}
		db.DPrintf(db.REALMD, "Link %v at %s\n", mnt, pn)
		if err := sc.MountService(pn, mnt, sp.NoLeaseId); err != nil {
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
	rm.realms[rid] = &Realm{named: p, sc: sc}
	return nil
}

func (rm *RealmSrv) Remove(ctx fs.CtxI, req proto.RemoveRequest, res *proto.RemoveResult) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	db.DPrintf(db.REALMD, "RealmSrv.Remove %v\n", req.Realm)
	rid := sp.Trealm(req.Realm)
	r, ok := rm.realms[rid]
	if !ok {
		return serr.MkErr(serr.TErrNotfound, rid)
	}

	if err := r.sc.RmDirEntries(sp.NAMED); err != nil {
		return err
	}

	// XXX remove root dir

	if err := rm.sc.Evict(r.named.GetPid()); err != nil {
		return err
	}
	delete(rm.realms, rid)
	return nil
}
