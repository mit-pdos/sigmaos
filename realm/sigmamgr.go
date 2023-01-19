package realm

import (
	"math"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/electclnt"
	"sigmaos/fslib"
	"sigmaos/machine"
	mproto "sigmaos/machine/proto"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/realm/proto"
	sp "sigmaos/sigmap"
	"sigmaos/stats"
)

type SigmaResourceMgr struct {
	sync.Mutex
	freeCoreGroups int64
	realmCreate    chan string
	realmDestroy   chan string
	realmmgrs      map[string]proc.Tpid
	anylc          map[string]bool // Used to enforce fairness between non-LC realms
	realmLocks     map[string]*electclnt.ElectClnt
	rclnts         map[string]*protdevclnt.ProtDevClnt
	pds            *protdevsrv.ProtDevSrv
	*procclnt.ProcClnt
	*config.ConfigClnt
	*fslib.FsLib
}

func MakeSigmaResourceMgr() *SigmaResourceMgr {
	m := &SigmaResourceMgr{}
	m.realmCreate = make(chan string)
	m.realmDestroy = make(chan string)
	var err error
	var mfs *memfssrv.MemFs
	mfs, m.FsLib, m.ProcClnt, err = memfssrv.MakeMemFs(sp.SIGMAMGR, "sigmamgr")
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}

	m.pds, err = protdevsrv.MakeProtDevSrvMemFs(mfs, m)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}

	// Mount the KPIDS dir.
	if err := procclnt.MountPids(m.FsLib, fslib.Named()); err != nil {
		db.DFatalf("Error mountpids: %v", err)
	}
	m.ConfigClnt = config.MakeConfigClnt(m.FsLib)
	m.initFS()
	m.realmLocks = make(map[string]*electclnt.ElectClnt)
	m.rclnts = make(map[string]*protdevclnt.ProtDevClnt)
	m.realmmgrs = make(map[string]proc.Tpid)
	m.anylc = make(map[string]bool)

	return m
}

// Make the initial realm dirs, and remove the unneeded union dirs.
func (m *SigmaResourceMgr) initFS() {
	dirs := []string{
		REALM_CONFIG,
		NODED_CONFIG,
		machine.MACHINES,
		REALM_NAMEDS,
		REALM_FENCES,
		REALM_MGRS,
	}
	for _, d := range dirs {
		if err := m.MkDir(d, 0777); err != nil {
			db.DFatalf("Error Mkdir %v in SigmaResourceMgr.initFs: %v", d, err)
		}
	}
}

func (m *SigmaResourceMgr) FreeCores(req mproto.MachineRequest, res *mproto.MachineResponse) error {
	m.freeCores(req.Ncores)
	db.DPrintf(db.SIGMAMGR, "free cores %v", req.Ncores)
	res.OK = true
	return nil
}

func (m *SigmaResourceMgr) FreeNode(req proto.SigmaMgrRequest, res *proto.SigmaMgrResponse) error {
	db.DFatalf("Free noded")
	return nil
}

func (m *SigmaResourceMgr) CreateRealm(req proto.SigmaMgrRequest, res *proto.SigmaMgrResponse) error {
	db.DPrintf(db.SIGMAMGR, "Create realm %v", req.RealmId)
	m.createRealm(req.RealmId)
	res.OK = true
	return nil
}

func (m *SigmaResourceMgr) DestroyRealm(req proto.SigmaMgrRequest, res *proto.SigmaMgrResponse) error {
	db.DPrintf(db.SIGMAMGR, "Destroy realm %v", req.RealmId)
	m.destroyRealm(req.RealmId)
	res.OK = true
	return nil
}

func (m *SigmaResourceMgr) RequestCores(req proto.SigmaMgrRequest, res *proto.SigmaMgrResponse) error {
	m.Lock()
	defer m.Unlock()
	db.DPrintf(db.SIGMAMGR, "[%v] Request cores hardreq %v", req.RealmId, req.HardReq)

	// If realm still exists, try to grow it.
	if _, ok := m.realmLocks[req.RealmId]; ok {
		m.growRealmL(req.RealmId, int(req.Qlen), req.MachineIds, req.HardReq)
	}
	res.OK = true
	return nil
}

// TODO: should probably release lock in this loop.
func (m *SigmaResourceMgr) tryGetFreeCores(nRetries int) bool {
	for i := 0; i < nRetries; i++ {
		if atomic.LoadInt64(&m.freeCoreGroups) > 0 {
			return true
		}
		db.DPrintf(db.SIGMAMGR, "Tried to get cores, but none free.")
		// TODO: parametrize?
		time.Sleep(10 * time.Millisecond)
	}
	db.DPrintf(db.SIGMAMGR, "Failed to find any free cores.")
	return false
}

func (m *SigmaResourceMgr) allocCores(realmId string, i int64) {
	atomic.AddInt64(&m.freeCoreGroups, -1*i)
	res := &proto.RealmMgrResponse{}
	req := &proto.RealmMgrRequest{
		Ncores: i,
	}
	err := m.rclnts[realmId].RPC("RealmMgr.GrantCores", req, res)
	if err != nil || !res.OK {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}
}

func (m *SigmaResourceMgr) freeCores(i int64) {
	atomic.AddInt64(&m.freeCoreGroups, i)
}

// Tries to add a Noded to a realm. Will first try and pull from the list of
// free Nodeds, and if none is available, it will try to make one free, and
// then retry. Caller holds lock.
func (m *SigmaResourceMgr) growRealmL(realmId string, qlen int, machines []string, hardReq bool) bool {
	// See if any cores are available.
	if m.tryGetFreeCores(1) {
		// Try to alloc qlen cores, or as many as are currently free otherwise.
		nallocd := int64(qlen)
		if nallocd == 0 {
			nallocd = 1
		}
		nfree := atomic.LoadInt64(&m.freeCoreGroups)
		if nfree < nallocd {
			nallocd = nfree
		}
		db.DPrintf(db.SIGMAMGR, "Allocate %v free cores to realm %v", nallocd, realmId)
		// Allocate cores to this realm.
		if nallocd > 0 {
			m.allocCores(realmId, nallocd)
			return true
		}
	}
	var ok bool
	var victimRealm string
	var nodedId string
	victimRealms := make([]string, 0)
	nodedIds := make([]string, 0)
	// No cores were available, so try to find a realm with spare resources.
	if len(machines) == 0 || !hardReq {
		db.DPrintf(db.SIGMAMGR, "[%v] search for cores without machine preference", realmId)
		victimRealm, _, ok = m.findOverProvisionedRealm(realmId, "")
		victimRealms = append(victimRealms, victimRealm)
		nodedIds = append(nodedIds, nodedId)
	} else {
		db.DPrintf(db.SIGMAMGR, "[%v] search for cores with machine preference %v", realmId, machines)
		var success bool
		for _, machine := range machines {
			db.DPrintf(db.SIGMAMGR, "[%v] search for cores on %v", realmId, machine)
			victimRealm, nodedId, success = m.findOverProvisionedRealm(realmId, machine)
			if success {
				victimRealms = append(victimRealms, victimRealm)
				nodedIds = append(nodedIds, nodedId)
				ok = true
			}
		}
	}
	if !ok {
		db.DPrintf(db.SIGMAMGR, "[%v] No overprovisioned realms available", realmId)
		return false
	}
	// Ask the over-provisioned realms to give up some cores.
	db.DPrintf(db.SIGMAMGR, "[%v] Requesting cores from realms %v nodeds %v", realmId, victimRealms, nodedIds)
	ok = false
	for i := range nodedIds {
		if ok = m.requestCores(victimRealms[i], nodedIds[i], hardReq); ok {
			// Wait for the over-provisioned realm to cede its cores.
			if m.tryGetFreeCores(50) {
				// Allocate core to this realm.
				m.allocCores(realmId, 1)
			}
		}
	}
	return ok
}

// Find an over-provisioned realm (a realm with resources to spare),
// specifically for a certain machine. Returns true if an overprovisioned realm
// was found, false otherwise.
func (m *SigmaResourceMgr) findOverProvisionedRealm(ignoreRealm string, machineId string) (opRealmId string, nodedId string, ok bool) {
	opRealmId = ""
	nodedId = ""
	ok = false
	m.ProcessDir(REALM_CONFIG, func(st *sp.Stat) (bool, error) {
		realmId := st.Name

		// Don't steal a noded from the requesting realm.
		if realmId == ignoreRealm {
			return false, nil
		}

		lock, exists := m.realmLocks[realmId]
		// If the realm we are looking at has been deleted, move on.
		if !exists {
			return false, nil
		}

		lockRealm(lock, realmId)
		defer unlockRealm(lock, realmId)

		rCfg := &RealmConfig{}
		m.ReadConfig(RealmConfPath(realmId), rCfg)

		// See if any nodeds have cores to spare.
		anyLC := false
		runningProcs := false
		overprovisioned := false
		for _, nodedId = range rCfg.NodedsAssigned {
			ndCfg := MakeNodedConfig()
			m.ReadConfig(NodedConfPath(nodedId), ndCfg)
			// If this machined is running on the node we care about, or the
			// requester has no preference
			if machineId == ndCfg.MachineId || machineId == "" {
				var op bool
				if anyLC, runningProcs, op = nodedOverprovisioned(m.FsLib, m.ConfigClnt, realmId, nodedId, db.SIGMAMGR); op {
					m.anylc[realmId] = m.anylc[realmId] || anyLC
					overprovisioned = true
					break
				}
			}
		}
		// If there are more than the minimum number of required Nodeds available...
		if len(rCfg.NodedsAssigned) > nReplicas() && overprovisioned {
			// If the realm we are requesting from has no LC procs, and neither does
			// the requesting realm, only steal if there is a resource imbalance (try
			// to enforce fairness).
			if !anyLC && !m.anylc[ignoreRealm] {
				stealerRCfg := &RealmConfig{}
				m.ReadConfig(RealmConfPath(ignoreRealm), stealerRCfg)
				// If the stealing realm has more (or as many) cores, and the victim is running procs, don't steal
				if stealerRCfg.NCores >= rCfg.NCores && runningProcs {
					return false, nil
				}
			}
			opRealmId = realmId
			ok = true
			return true, nil
		}
		return false, nil
	})
	return opRealmId, nodedId, ok
}

// Ascertain whether or not a noded is overprovisioned.
//
// XXX Eventually, we'll want to find overprovisioned realms according to
// more nuanced metrics, e.g. how many Nodeds are running procs that hold
// state, etc.
func nodedOverprovisioned(fsl *fslib.FsLib, cc *config.ConfigClnt, realmId string, nodedId string, debug db.Tselector) (anylc bool, runningprocs bool, overprov bool) {
	ndCfg := MakeNodedConfig()
	cc.ReadConfig(NodedConfPath(nodedId), ndCfg)
	db.DPrintf(debug, "[%v] Check noded %v utilization", realmId, nodedId)
	s := &stats.StatInfo{}
	err := fsl.GetFileJson(path.Join(RealmPath(realmId), sp.PROCDREL, ndCfg.ProcdIp, sp.STATSD), s)
	// Only overprovisioned if hasn't shut down/crashed.
	if err != nil {
		db.DPrintf(debug+db.ERR, "Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", err)
		return false, false, false
	}
	// Count the total number of cores assigned to this noded.
	totalCores := 0.0
	for _, cores := range ndCfg.Cores {
		totalCores += float64(cores.Size())
	}
	nLCCoresUsed := s.CustomUtil / 100.0
	//	nLCCoresUsed := s.CustomUtil / 100.0
	// Count how many cores we would revoke.
	coresToRevoke := float64(ndCfg.Cores[len(ndCfg.Cores)-1].Size())
	// If we don't have >= 1 core group to spare for LC procs, we aren't
	// overprovisioned
	thresh := sp.Conf.Realm.GROW_CPU_UTIL_THRESHOLD / 100.0
	if nLCCoresUsed > 0 {
		const buffer float64 = 0.2
		proposedNCores := totalCores - coresToRevoke
		// Ratio by which load will increase if the proposed revocation happens.
		loadIncRatio := totalCores / proposedNCores
		// cload is additive CPU usage of LC procs.
		cload := math.Max(s.CustomUtil, s.CustomLoad[0]) / 100.0 / totalCores
		// util is the % of all CPUs being used.
		util := math.Max(s.Util, s.Load[0]) / 100.0
		// If we push above this threshold, we'll just have to re-grow again.
		// So, avoid doing so unnecessarily.
		// 1/2 core buffer.
		db.DPrintf(debug, "[%v] noded %v stats Util:%v Load: %v, CustomUtil:%v, CustomLoad:%v", realmId, nodedId, s.Util, s.Load, s.CustomUtil, s.CustomLoad)
		db.DPrintf(debug, "[%v] noded %v derived stats util:%v cload:%v, lir:%v", realmId, nodedId, util, cload, loadIncRatio)
		if cload*loadIncRatio+buffer >= thresh || util*loadIncRatio+buffer >= thresh {
			db.DPrintf(debug, "[%v] Noded is using LC cores well, not overprovisioned: tc %v ctr %v ncl %v nu %v", realmId, totalCores, coresToRevoke, cload*loadIncRatio >= thresh, util*loadIncRatio >= thresh)
			return true, true, false
		}
	}
	db.DPrintf(debug, "[%v] Noded %v is underutilized threshold %v, had %v cores remaining.", realmId, nodedId, thresh, len(ndCfg.Cores))
	// Don't evict this noded if it is running any LC procs.
	if len(ndCfg.Cores) == 1 {
		qs := []string{sp.PROCD_RUNQ_LC}
		for _, q := range qs {
			queued, err := fsl.GetDir(path.Join(RealmPath(realmId), sp.PROCDREL, ndCfg.ProcdIp, q))
			if err != nil {
				db.DPrintf(debug+db.ERR, "Couldn't get queue dir %v: %v", q, err)
				return false, false, false
			}
			// If there are LC procs queued, don't shrink.
			if len(queued) > 0 {
				db.DPrintf(debug, "Can't evict noded, had %v queued LC procs", len(queued))
				return true, true, false
			}
		}
		runningProcs, err := fsl.GetDir(path.Join(RealmPath(realmId), sp.PROCDREL, ndCfg.ProcdIp, sp.PROCD_RUNNING))
		if err != nil {
			db.DPrintf(debug+db.ERR, "Couldn't get procs running dir: %v", err)
			return false, false, false
		}
		// If this is the last core group for this noded, and its utilization is over
		// a certain threshold (and it is running procs), don't evict.
		if s.Util >= sp.Conf.Realm.SHRINK_CPU_UTIL_THRESHOLD && len(runningProcs) > 0 {
			db.DPrintf(debug, "[%v] Can't evict noded, util: %v runningProcs: %v", realmId, s.Util, len(runningProcs))
			return false, true, false
		}
		for _, st := range runningProcs {
			p := proc.MakeEmptyProc()
			err := fsl.GetFileJson(path.Join(RealmPath(realmId), sp.PROCDREL, ndCfg.ProcdIp, sp.PROCD_RUNNING, st.Name), p)
			if err != nil {
				continue
			}
			// If this is a LC proc, return false.
			if p.GetType() == proc.T_LC {
				db.DPrintf(debug, "Can't evict noded, running LC proc")
				return true, true, false
			} else {
				db.DPrintf(debug, "Noded %v's proc %v, is not LC", nodedId, p)
			}
		}
		db.DPrintf(debug, "[%v] Evicting noded %v realm %v", realmId, nodedId, realmId)
	} else {
		runningProcs, err := fsl.GetDir(path.Join(RealmPath(realmId), sp.PROCDREL, ndCfg.ProcdIp, sp.PROCD_RUNNING))
		if err != nil {
			db.DPrintf(debug+db.ERR, "Couldn't get procs running dir: %v", err)
			return false, false, false
		}
		if len(runningProcs) > 3 {
			return false, true, true
		}
	}
	return false, false, true
}

// Create a realm.
func (m *SigmaResourceMgr) createRealm(realmId string) {
	m.Lock()
	defer m.Unlock()

	// Make sure we haven't created this realm before.
	if _, ok := m.realmLocks[realmId]; ok {
		db.DFatalf("tried to create realm twice %v", realmId)
	}
	m.realmLocks[realmId] = electclnt.MakeElectClnt(m.FsLib, realmFencePath(realmId), 0777)

	lockRealm(m.realmLocks[realmId], realmId)

	cfg := &RealmConfig{}
	cfg.Rid = realmId

	// Make the realm config file.
	m.WriteConfig(RealmConfPath(realmId), cfg)

	unlockRealm(m.realmLocks[realmId], realmId)

	// Start this realm's realmmgr.
	m.startRealmMgr(realmId)
}

// Request a Noded from realm realmId.
func (m *SigmaResourceMgr) requestCores(realmId string, nodedId string, hardReq bool) bool {
	db.DPrintf(db.SIGMAMGR, "Sigmamgr requesting cores from %v", realmId)
	res := &proto.RealmMgrResponse{}
	req := &proto.RealmMgrRequest{
		Ncores:  1,
		HardReq: hardReq,
		NodedId: nodedId,
	}
	err := m.rclnts[realmId].RPC("RealmMgr.RevokeCores", req, res)
	if err != nil {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}
	db.DPrintf(db.SIGMAMGR, "Sigmamgr done requesting cores from %v", realmId)
	return res.OK
}

// Destroy a realm.
func (m *SigmaResourceMgr) destroyRealm(realmId string) {
	m.Lock()
	defer m.Unlock()

	db.DPrintf(db.SIGMAMGR, "Destroy realm %v", realmId)

	lockRealm(m.realmLocks[realmId], realmId)

	// Update the realm config to note that the realm is being shut down.
	cfg := &RealmConfig{}
	m.ReadConfig(RealmConfPath(realmId), cfg)
	cfg.Shutdown = true
	m.WriteConfig(RealmConfPath(realmId), cfg)

	unlockRealm(m.realmLocks[realmId], realmId)
	delete(m.realmLocks, realmId)

	res := &proto.RealmMgrResponse{}
	req := &proto.RealmMgrRequest{
		AllCores: true,
	}
	err := m.rclnts[realmId].RPC("RealmMgr.ShutdownRealm", req, res)
	if err != nil || !res.OK {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}

	m.evictRealmMgr(realmId)
	db.DPrintf(db.SIGMAMGR, "Done destroying realm %v", realmId)
}

func (m *SigmaResourceMgr) startRealmMgr(realmId string) {
	pid := proc.Tpid("realmmgr-" + proc.GenPid().String())
	p := proc.MakeProcPid(pid, "realm/realmmgr", []string{realmId})
	if _, err := m.SpawnKernelProc(p, fslib.Named(), realmId, procclnt.HLINUX); err != nil {
		db.DFatalf("Error spawn realmmgr %v", err)
	}
	if err := m.WaitStart(p.GetPid()); err != nil {
		db.DFatalf("Error WaitStart realmmgr %v", err)
	}
	db.DPrintf(db.SIGMAMGR, "Sigmamgr started realmmgr %v in realm %v", pid.String(), realmId)
	m.realmmgrs[realmId] = pid
	var err error
	m.rclnts[realmId], err = protdevclnt.MkProtDevClnt(m.FsLib, realmMgrPath(realmId))
	if err != nil {
		db.DFatalf("Error MkProtDevClnt: %v", err)
	}
}

func (m *SigmaResourceMgr) evictRealmMgr(realmId string) {
	pid := m.realmmgrs[realmId]
	db.DPrintf(db.SIGMAMGR, "Sigmamgr evicting realmmgr %v in realm %v", pid.String(), realmId)
	if err := m.Evict(pid); err != nil {
		db.DFatalf("Error evict realmmgr %v for realm %v", pid, realmId)
	}
	if status, err := m.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
		db.DFatalf("Error bad status evict realmmgr %v for realm %v: status %v err %v", pid, realmId, status, err)
	}
	delete(m.realmmgrs, realmId)
	delete(m.rclnts, realmId)
}

func (m *SigmaResourceMgr) Work() {
	m.pds.Serve()
	m.pds.Done()
}
