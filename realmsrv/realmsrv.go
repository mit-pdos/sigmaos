package realmsrv

import (
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/procqclnt"
	"sigmaos/realmsrv/proto"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	MKNET      = "./bin/kernel/create-net.sh"
	MIN_PORT   = 30000
	NAMED_MCPU = 0
)

// Fairness
const (
	FAIRNESS_CHECK_PERIOD = time.Second
	N_SAMPLE              = 2
	STARVATION_RATIO      = 0.1
)

type Realm struct {
	named *proc.Proc // XXX groupmgr for fault tolerance
	sc    *sigmaclnt.SigmaClnt
}

type RealmSrv struct {
	mu         sync.Mutex
	realms     map[sp.Trealm]*Realm
	sc         *sigmaclnt.SigmaClnt
	pq         *procqclnt.ProcQClnt
	sd         *scheddclnt.ScheddClnt
	lastNDPort int
	ch         chan struct{}
}

func RunRealmSrv() error {
	rs := &RealmSrv{
		lastNDPort: MIN_PORT,
		realms:     make(map[sp.Trealm]*Realm),
	}
	rs.ch = make(chan struct{})
	db.DPrintf(db.REALMD, "Run %v %s\n", sp.REALMD, os.Environ())
	pcfg := proc.GetProcEnv()
	ssrv, err := sigmasrv.NewSigmaSrv(sp.REALMD, rs, pcfg)
	if err != nil {
		return err
	}
	_, serr := ssrv.MemFs.Create(sp.REALMSREL, 0777|sp.DMDIR, sp.OREAD, sp.NoLeaseId)
	if serr != nil {
		return serr
	}
	db.DPrintf(db.REALMD, "newsrv ok")
	rs.sc = ssrv.MemFs.SigmaClnt()
	rs.pq = procqclnt.NewProcQClnt(rs.sc.FsLib)
	rs.sd = scheddclnt.NewScheddClnt(rs.sc.FsLib)
	go rs.enforceResourcePolicy()
	err = ssrv.RunServer()
	return nil
}

func NewNet(net string) error {
	if net == "" {
		return nil
	}
	args := []string{"sigmanet-" + net}
	out, err := exec.Command(MKNET, args...).Output()
	if err != nil {
		db.DPrintf(db.REALMD, "NewNet: %v %s err %v\n", net, string(out), err)
		return err
	}
	db.DPrintf(db.REALMD, "NewNet: %v\n", string(out))
	return nil
}

// XXX clean up if fail during Make
func (rm *RealmSrv) Make(ctx fs.CtxI, req proto.MakeRequest, res *proto.MakeResult) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	db.DPrintf(db.REALMD, "RealmSrv.Make %v %v", req.Realm, req.Network)
	rid := sp.Trealm(req.Realm)
	// If realm already exists
	if _, ok := rm.realms[rid]; ok {
		return serr.NewErr(serr.TErrExists, rid)
	}
	if err := NewNet(req.Network); err != nil {
		return err
	}

	p := proc.NewProc("named", []string{req.Realm, "0"})
	p.SetMcpu(NAMED_MCPU)

	db.DPrintf(db.REALMD, "RealmSrv.Make %v spawn named", req.Realm)
	if _, errs := rm.sc.SpawnBurst([]*proc.Proc{p}, 2); len(errs) != 0 {
		db.DPrintf(db.REALMD_ERR, "Error SpawnBurst: %v", errs[0])
		return errs[0]
	}
	if err := rm.sc.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.REALMD_ERR, "Error WaitStart: %v", err)
		return err
	}
	db.DPrintf(db.REALMD, "RealmSrv.Make %v named started", req.Realm)

	// wait until realm's named is ready to serve
	sem := semclnt.NewSemClnt(rm.sc.FsLib, path.Join(sp.REALMS, req.Realm)+".sem")
	if err := sem.Down(); err != nil {
		return err
	}

	db.DPrintf(db.REALMD, "RealmSrv.Make named ready to serve for %v", rid)

	pcfg := proc.NewDifferentRealmProcEnv(rm.sc.ProcEnv(), rid)
	sc, err := sigmaclnt.NewSigmaClntFsLib(pcfg)
	if err != nil {
		db.DPrintf(db.REALMD_ERR, "Error NewSigmaClntRealm: %v", err)
		return err
	}
	// Make some rootrealm services available in new realm
	namedMount := rm.sc.GetNamedMount()
	for _, s := range []string{sp.LCSCHEDREL, sp.PROCQREL, sp.SCHEDDREL, sp.UXREL, sp.S3REL, sp.DBREL, sp.BOOTREL, sp.MONGOREL} {
		pn := path.Join(sp.NAMED, s)
		mnt := sp.Tmount{Addr: namedMount.Addr, Root: s}
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
		return serr.NewErr(serr.TErrNotfound, rid)
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

func (rm *RealmSrv) realmResourceUsage(running map[sp.Trealm][]*proc.Proc) map[sp.Trealm]proc.Tmem {
	memUsage := make(map[sp.Trealm]proc.Tmem)

	rm.mu.Lock()
	// Initialize from realmmgr's map, since a realm may have never spawn a proc
	// (and hence not show up in any schedd samples) but may still be starved.
	for r, _ := range rm.realms {
		// Don't consider the root realm when thinking about starvation
		if r != sp.ROOTREALM {
			memUsage[r] = proc.Tmem(0)
		}
	}
	rm.mu.Unlock()

	// Sum memory reservations across machines.
	for r, ps := range running {
		for _, p := range ps {
			if p.GetType() != proc.T_BE {
				continue
			}
			memUsage[r] += p.GetMem()
		}
	}

	return memUsage
}

func findStarvedRealms(rusage map[sp.Trealm]proc.Tmem) (sp.Trealm, []sp.Trealm) {
	starved := make([]sp.Trealm, 0)
	// Find the realm with the most resource utilization.
	maxRealm := sp.Trealm("")
	maxUsage := proc.Tmem(0)
	for r, u := range rusage {
		if maxUsage < u || maxUsage == 0 {
			maxRealm = r
			maxUsage = u
		}
	}

	// If no realms are using any resources, then no realms are starved
	if maxUsage == 0 {
		return maxRealm, starved
	}

	for r, u := range rusage {
		// If a realm is using less than STARVATION_RATIO fraction of the max
		// realm's resources, it is a candidate for a starvation check.
		if float64(u)/float64(maxUsage) < STARVATION_RATIO {
			starved = append(starved, r)
		}
	}
	return maxRealm, starved
}

// Check if a queue has built up for a starved realm
func queueBuildup(starved []sp.Trealm, qlens map[sp.Trealm]int) bool {
	for _, r := range starved {
		if qlens[r] > 0 {
			return true
		}
	}
	return false
}

func selectVictim(ps []*proc.Proc) *proc.Proc {
	var maxProc *proc.Proc
	var maxMem proc.Tmem
	for _, p := range ps {
		if p.GetMem() > maxMem || maxMem == 0 {
			maxProc = p
			maxMem = p.GetMem()
		}
	}
	return maxProc
}

func (rm *RealmSrv) enforceResourcePolicy() {
	t := time.NewTicker(FAIRNESS_CHECK_PERIOD)
	for {
		<-t.C
		db.DPrintf(db.FAIRNESS, "Check BE resource allocation")
		running, err := rm.sd.GetRunningProcs(N_SAMPLE)
		if err != nil {
			db.DFatalf("Err getting running procs: %v", err)
		}
		db.DPrintf(db.FAIRNESS, "Running procs: %v", running)
		resourceUsage := rm.realmResourceUsage(running)
		db.DPrintf(db.FAIRNESS, "Resource usage: %v", running)
		maxRealm, starvedRealms := findStarvedRealms(resourceUsage)
		db.DPrintf(db.FAIRNESS, "Starved realms: %v", starvedRealms)
		// If there are no starved realms, continue
		if len(starvedRealms) == 0 {
			db.DPrintf(db.FAIRNESS, "No starved realms. Fairness achieved.")
			continue
		}
		realmQLens, err := rm.pq.GetQueueStats(N_SAMPLE)
		if err != nil {
			db.DFatalf("Err getting queue stats: %v", err)
		}
		db.DPrintf(db.FAIRNESS, "Realm qlens: %v", realmQLens)
		if !queueBuildup(starvedRealms, realmQLens) {
			db.DPrintf(db.FAIRNESS, "No queue buildup. Fairness achieved.")
			continue
		}
		db.DPrintf(db.FAIRNESS, "Queue buildup. Selecting a victim")
		victim := selectVictim(running[maxRealm])
		db.DPrintf(db.FAIRNESS, "Victim realm %v proc %v", victim.GetRealm(), victim)
		rm.sc.EvictRealmProc(victim.GetPid(), victim.GetKernelID())
	}
}
