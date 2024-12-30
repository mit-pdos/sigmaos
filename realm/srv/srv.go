package srv

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	kernelclnt "sigmaos/kernel/clnt"
	"sigmaos/proc"
	realmpkg "sigmaos/realm"
	"sigmaos/realm/proto"
	"sigmaos/rpc"
	beschedclnt "sigmaos/sched/besched/clnt"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	spprotosrv "sigmaos/spproto/srv"
	"sigmaos/util/coordination/semaphore"
)

const (
	MKNET      = "./bin/kernel/create-net.sh"
	NAMED_MCPU = 0
)

type Subsystem struct {
	kernelID string
	pid      sp.Tpid
}

type Realm struct {
	sync.Mutex
	named                    *proc.Proc // XXX groupmgr for fault tolerance
	perRealmKernelSubsystems []*Subsystem
	sc                       *sigmaclnt.SigmaClnt
}

func newRealm() *Realm {
	return &Realm{
		perRealmKernelSubsystems: []*Subsystem{},
	}
}

func (r *Realm) addSubsystem(kernelID string, pid sp.Tpid) {
	r.Lock()
	defer r.Unlock()

	r.perRealmKernelSubsystems = append(r.perRealmKernelSubsystems, &Subsystem{
		kernelID: kernelID,
		pid:      pid,
	})
}

type RealmSrv struct {
	mu        sync.Mutex
	dialproxy bool
	realms    map[sp.Trealm]*Realm
	sc        *sigmaclnt.SigmaClntKernel
	be        *beschedclnt.BESchedClnt
	sd        *mschedclnt.MSchedClnt
	mkc       *kernelclnt.MultiKernelClnt
	ch        chan struct{}
}

func RunRealmSrv(dialproxy bool) error {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	sc.GetDialProxyClnt().AllowConnectionsFromAllRealms()
	rs := &RealmSrv{
		dialproxy: dialproxy,
		realms:    make(map[sp.Trealm]*Realm),
	}
	rs.ch = make(chan struct{})
	db.DPrintf(db.REALMD, "Run %v %s\n", sp.REALMD, os.Environ())
	if false {
		allowedPaths := []string{sp.REALMSREL, rpc.RPC}
		ssrv, err := sigmasrv.NewSigmaSrvClntAuthFn(sp.REALMD, sc, rs, spprotosrv.AttachAllowAllPrincipalsSelectPaths(allowedPaths))
		_ = ssrv
		_ = err
	}
	ssrv, err := sigmasrv.NewSigmaSrvClntAuthFn(sp.REALMD, sc, rs, spprotosrv.AttachAllowAllToAll)
	if err != nil {
		return err
	}
	_, serr := ssrv.MemFs.Create(sp.REALMSREL, 0777|sp.DMDIR, sp.OREAD, sp.NoLeaseId)
	if serr != nil {
		return serr
	}
	db.DPrintf(db.REALMD, "newsrv ok")
	rs.sc = sigmaclnt.NewSigmaClntKernel(ssrv.MemFs.SigmaClnt())
	rs.mkc = kernelclnt.NewMultiKernelClnt(ssrv.MemFs.SigmaClnt().FsLib, db.REALMD, db.REALMD_ERR)
	rs.be = beschedclnt.NewBESchedClnt(rs.sc.FsLib)
	rs.sd = mschedclnt.NewMSchedClnt(rs.sc.FsLib, pe.GetKernelID())
	go rs.enforceResourcePolicy()
	err = ssrv.RunServer()
	rs.mkc.StopWatching()
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
func (rm *RealmSrv) Make(ctx fs.CtxI, req proto.MakeReq, res *proto.MakeRep) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	db.DPrintf(db.REALMD, "RealmSrv.Make %v %v", req.Realm, req.Network)
	defer db.DPrintf(db.REALMD, "RealmSrv.Make done %v %v", req.Realm, req.Network)
	rid := sp.Trealm(req.Realm)
	// If realm already exists
	if _, ok := rm.realms[rid]; ok {
		return serr.NewErr(serr.TErrExists, rid)
	}
	if err := NewNet(req.Network); err != nil {
		return err
	}
	r := newRealm()
	p := proc.NewProc("named", []string{req.Realm})
	// Set up a realm switch: when named runs, it should start as a member of the
	// new realm.
	p.SetRealmSwitch(rid)
	// Make sure named uses dialproxy
	p.GetProcEnv().UseDialProxy = rm.dialproxy
	p.SetMcpu(NAMED_MCPU)
	r.named = p

	db.DPrintf(db.REALMD, "RealmSrv.Make %v spawn named %v", req.Realm, p)
	if err := rm.sc.Spawn(p); err != nil {
		db.DPrintf(db.REALMD_ERR, "Error SpawnBurst: %v", err)
		return err
	}
	if err := rm.sc.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.REALMD_ERR, "Error WaitStart: %v", err)
		return err
	}
	db.DPrintf(db.REALMD, "RealmSrv.Make %v named started", req.Realm)

	// wait until realm's named is ready to serve
	sem := semaphore.NewSemaphore(rm.sc.FsLib, filepath.Join(sp.REALMS, req.Realm)+".sem")
	if err := sem.Down(); err != nil {
		return err
	}

	db.DPrintf(db.REALMD, "RealmSrv.Make named ready to serve for %v", rid)
	pe := proc.NewDifferentRealmProcEnv(rm.sc.ProcEnv(), rid)
	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	if err != nil {
		db.DPrintf(db.REALMD_ERR, "Error NewSigmaClntRealm: %v", err)
		return err
	}
	r.sc = sc
	// Make some rootrealm services available in new realm
	rootNamedEP, err := rm.sc.GetNamedEndpoint()
	if err != nil {
		db.DPrintf(db.ERROR, "Error GetNamedEndpoint: %v", err)
		return err
	}
	// Export some service dirs from root realm to the new realm by
	// making endpoints for them in this realm.
	for s, _ := range sp.RootNamedMountedDirs {
		pn := filepath.Join(sp.NAMED, s)
		ep := sp.NewEndpoint(sp.INTERNAL_EP, rootNamedEP.Addrs())
		ep.SetTree(s)
		db.DPrintf(db.REALMD, "Link %v at %s\n", ep, pn)
		if err := sc.MkEndpointFile(pn, ep); err != nil {
			db.DPrintf(db.ERROR, "EndpointService %v err %v\n", pn, err)
			return err
		}
	}
	// Make some realm dirs
	for _, s := range []string{sp.KPIDSREL, sp.S3REL, sp.UXREL} {
		pn := filepath.Join(sp.NAMED, s)
		db.DPrintf(db.REALMD, "Mkdir %v", pn)
		if err := sc.MkDir(pn, 0777); err != nil {
			db.DPrintf(db.REALMD, "EndpointService %v err %v\n", pn, err)
			return err
		}
	}

	errC := make(chan error)
	// Spawn per-realm kernel procs
	go func() {
		if err := rm.bootPerRealmKernelSubsystems(r, sp.Trealm(req.Realm), sp.S3REL, req.GetNumS3()); err != nil {
			db.DPrintf(db.ERROR, "Error boot per realm [%v] subsystems: %v", sp.S3REL, err)
			errC <- err
			return
		}
		errC <- nil
	}()
	go func() {
		if err := rm.bootPerRealmKernelSubsystems(r, sp.Trealm(req.Realm), sp.UXREL, req.GetNumUX()); err != nil {
			db.DPrintf(db.ERROR, "Error boot per realm [%v] subsystems: %v", sp.UXREL, err)
			errC <- err
			return
		}
		errC <- nil
	}()
	for i := 0; i < 2; i++ {
		if err := <-errC; err != nil {
			db.DPrintf(db.ERROR, "Error MakeRealm [%v] when booting subsystems", sp.Trealm(req.Realm))
			return err
		}
	}
	rm.realms[rid] = r
	return nil
}

func (rm *RealmSrv) Remove(ctx fs.CtxI, req proto.RemoveReq, res *proto.RemoveRep) error {
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

	db.DPrintf(db.REALMD, "[%v] Remove realm, subsystems %v", rid, r.perRealmKernelSubsystems)

	for _, ss := range r.perRealmKernelSubsystems {
		if err := rm.mkc.EvictKernelProc(ss.kernelID, ss.pid); err != nil {
			db.DPrintf(db.ERROR, "Error Evict per-realm kernel subsystem: %v", err)
			return err
		}
	}

	// XXX remove root dir

	if err := rm.sc.Evict(r.named.GetPid()); err != nil {
		db.DPrintf(db.ERROR, "Error Evict realm named: %v", err)
		return err
	}
	if _, err := rm.sc.WaitExit(r.named.GetPid()); err != nil {
		db.DPrintf(db.ERROR, "Error WaitExit realm named: %v", err)
		return err
	}
	delete(rm.realms, rid)
	return nil
}

func (rm *RealmSrv) bootPerRealmKernelSubsystems(r *Realm, realm sp.Trealm, ss string, n int64) error {
	db.DPrintf(db.REALMD, "[%v] boot per-kernel subsystems [%v] n %v", realm, ss, n)
	defer db.DPrintf(db.REALMD, "[%v] boot per-kernel subsystems done [%v] n %v", realm, ss, n)
	kernels, err := rm.mkc.GetGeneralKernels()
	db.DPrintf(db.REALMD, "%v: [%v] kernels %v %v\n", realm, ss, kernels, err)
	if err != nil {
		return err
	}
	if int64(len(kernels)) < n {
		db.DPrintf(db.ERROR, "Tried to boot more than one kernel subsystem per kernel")
		return fmt.Errorf("Tried to boot more than one kernel subsystem per kernel")
	}
	if n == realmpkg.SUBSYSTEM_PER_NODE {
		// Boot one subsystem for the realm on each node in the deployment, so use
		// the full slice of kernels
	} else {
		// Shuffle the slice of kernels
		for i := range kernels {
			j := rand.Intn(i + 1)
			kernels[i], kernels[j] = kernels[j], kernels[i]
		}
		// Take the first N of them
		kernels = kernels[:n]
	}
	db.DPrintf(db.REALMD, "[%v] boot per-kernel subsystems selected kernels: %v", realm, kernels)
	done := make(chan error)
	for _, kid := range kernels {
		go func(kid string) {
			pid, err := rm.mkc.BootInRealm(kid, realm, ss, nil)
			if err != nil {
				db.DPrintf(db.ERROR, "Error boot subsystem %v in realm %v on kid %v: %v", ss, realm, kid, err)
				done <- err
			} else {
				r.addSubsystem(kid, pid)
			}
			done <- nil
		}(kid)
	}
	var bootErr error
	for _ = range kernels {
		if e := <-done; e != nil {
			bootErr = e
		}
	}
	return bootErr
}

func (rm *RealmSrv) realmResourceUsage(running map[sp.Trealm][]*proc.Proc) map[sp.Trealm]proc.Tmem {
	memUsage := make(map[sp.Trealm]proc.Tmem)

	rm.mu.Lock()
	// Initialize from realmmgr's map, since a realm may have never spawn a proc
	// (and hence not show up in any msched samples) but may still be starved.
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
		if float64(u)/float64(maxUsage) < sp.Conf.Realm.STARVATION_RATIO {
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
	t := time.NewTicker(sp.Conf.Realm.FAIRNESS_CHECK_PERIOD)
	for {
		<-t.C
		db.DPrintf(db.FAIRNESS, "Check BE resource allocation")
		running, err := rm.sd.GetRunningProcs(sp.Conf.Realm.N_SAMPLE)
		if err != nil {
			db.DPrintf(db.ERROR, "Err getting running procs: %v", err)
			continue
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
		realmQLens, err := rm.be.GetQueueStats(sp.Conf.Realm.N_SAMPLE)
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
