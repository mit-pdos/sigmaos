package realm

import (
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ulambda/config"
	db "ulambda/debug"
	"ulambda/electclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/machine"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/resource"
	"ulambda/stats"
)

type SigmaResourceMgr struct {
	sync.Mutex
	freeCoreGroups int64
	realmCreate    chan string
	realmDestroy   chan string
	realmmgrs      map[string]proc.Tpid
	realmLocks     map[string]*electclnt.ElectClnt
	*procclnt.ProcClnt
	*config.ConfigClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func MakeSigmaResourceMgr() *SigmaResourceMgr {
	m := &SigmaResourceMgr{}
	m.realmCreate = make(chan string)
	m.realmDestroy = make(chan string)
	var err error
	m.MemFs, m.FsLib, m.ProcClnt, err = fslibsrv.MakeMemFs(np.SIGMAMGR, "sigmamgr")
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	// Mount the KPIDS dir.
	if err := procclnt.MountPids(m.FsLib, fslib.Named()); err != nil {
		db.DFatalf("Error mountpids: %v", err)
	}
	m.ConfigClnt = config.MakeConfigClnt(m.FsLib)
	m.initFS()
	resource.MakeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, m.Root(), np.RESOURCE_CTL)
	m.realmLocks = make(map[string]*electclnt.ElectClnt)
	m.realmmgrs = make(map[string]proc.Tpid)

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

func (m *SigmaResourceMgr) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Trealm:
		m.destroyRealm(msg.Name)
	case resource.Tnode:
		db.DPrintf("SIGMAMGR", "free noded %v", msg.Name)
	case resource.Tcore:
		m.freeCores(1)
		db.DPrintf("SIGMAMGR", "free cores %v", msg.Name)
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// Handle a resource request.
func (m *SigmaResourceMgr) handleResourceRequest(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Trealm:
		m.createRealm(msg.Name)
	case resource.Tcore:
		m.Lock()
		defer m.Unlock()

		realmId := msg.Name
		// If realm still exists, try to grow it.
		if _, ok := m.realmLocks[realmId]; ok {
			m.growRealmL(realmId)
		}
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// TODO: should probably release lock in this loop.
func (m *SigmaResourceMgr) tryGetFreeCores(nRetries int) bool {
	for i := 0; i < nRetries; i++ {
		if atomic.LoadInt64(&m.freeCoreGroups) > 0 {
			return true
		}
		db.DPrintf("SIGMAMGR", "Tried to get cores, but none free.")
		// TODO: parametrize?
		time.Sleep(10 * time.Millisecond)
	}
	db.DPrintf("SIGMAMGR", "Failed to find any free cores.")
	return false
}

func (m *SigmaResourceMgr) allocCores(realmId string, i int64) {
	atomic.AddInt64(&m.freeCoreGroups, -1*i)
	msg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, "", 1)
	resource.SendMsg(m.FsLib, path.Join(realmMgrPath(realmId), np.RESOURCE_CTL), msg)
}

func (m *SigmaResourceMgr) freeCores(i int64) {
	atomic.AddInt64(&m.freeCoreGroups, i)
}

// Tries to add a Noded to a realm. Will first try and pull from the list of
// free Nodeds, and if none is available, it will try to make one free, and
// then retry. Caller holds lock.
func (m *SigmaResourceMgr) growRealmL(realmId string) bool {
	// See if any cores are available.
	if m.tryGetFreeCores(1) {
		// Allocate cores to this realm.
		m.allocCores(realmId, 1)
		return true
	}
	// No cores were available, so try to find a realm with spare resources.
	opRealmId, ok := m.findOverProvisionedRealm(realmId)
	if !ok {
		db.DPrintf("SIGMAMGR", "No overprovisioned realms available")
		return false
	}
	// Ask the over-provisioned realm to give up some cores.
	m.requestCores(opRealmId)
	// Wait for the over-provisioned realm to cede its cores.
	if m.tryGetFreeCores(100) {
		// Allocate cores to this realm.
		m.allocCores(realmId, 1)
		return true
	}
	return false
}

// Ascertain whether or not a noded is overprovisioned.
//
// XXX Eventually, we'll want to find overprovisioned realms according to
// more nuanced metrics, e.g. how many Nodeds are running procs that hold
// state, etc.
func (m *SigmaResourceMgr) nodedOverprovisioned(realmId string, nodedId string) bool {
	ndCfg := MakeNodedConfig()
	m.ReadConfig(NodedConfPath(nodedId), ndCfg)
	db.DPrintf("SIGMAMGR", "Check if noded %v realm %v is overprovisioned", nodedId, realmId)
	s := &stats.StatInfo{}
	err := m.GetFileJson(path.Join(RealmPath(realmId), np.PROCDREL, ndCfg.ProcdIp, np.STATSD), s)
	// Only overprovisioned if hasn't shut down/crashed.
	if err != nil {
		db.DPrintf("SIGMAMGR_ERR", "Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", err)
		db.DPrintf(db.ALWAYS, "Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", err)
		return false
	}
	// Count the total number of cores assigned to this noded.
	totalCores := 0.0
	for _, cores := range ndCfg.Cores {
		totalCores += float64(cores.Size())
	}
	nLCCoresUsed := s.CustomUtil / 100.0
	// Count how many cores we would revoke.
	coresToRevoke := float64(ndCfg.Cores[len(ndCfg.Cores)-1].Size())
	// If we don't have >= 1 core group to spare for LC procs, we aren't
	// overprovisioned
	if totalCores-coresToRevoke < nLCCoresUsed {
		db.DPrintf("SIGMAMGR", "Noded is using LC cores well, not overprovisioned: %v - %v >= %v", totalCores, coresToRevoke, nLCCoresUsed)
		return false
	}
	// Don't evict this noded if it is running any LC procs.
	if len(ndCfg.Cores) == 1 {
		runningProcs, err := m.GetDir(path.Join(RealmPath(realmId), np.PROCDREL, ndCfg.ProcdIp, np.PROCD_RUNNING))
		if err != nil {
			db.DPrintf(db.ALWAYS, "Couldn't get procs running dir: %v", err)
			return false
		}
		// If this is the last core group for this noded, and its utilization is over
		// a certain threshold (and it is running procs), don't evict.
		if s.Util >= np.Conf.Realm.SHRINK_CPU_UTIL_THRESHOLD && len(runningProcs) > 0 {
			db.DPrintf("SIGMAMGR", "Can't evict noded, util: %v runningProcs: %v", s.Util, len(runningProcs))
			return false
		}
		for _, st := range runningProcs {
			b, err := m.GetFile(path.Join(RealmPath(realmId), np.PROCDREL, ndCfg.ProcdIp, np.PROCD_RUNNING, st.Name))
			if err != nil {
				continue
			}
			// If this is a LC proc, return false.
			if strings.Contains(string(b), "T_LC") {
				db.DPrintf("SIGMAMGR", "Can't evict noded, running LC proc")
				return false
			}
		}
	}
	return true
}

// Find an over-provisioned realm (a realm with resources to spare). Returns
// true if an overprovisioned realm was found, false otherwise.
func (m *SigmaResourceMgr) findOverProvisionedRealm(ignoreRealm string) (string, bool) {
	opRealmId := ""
	ok := false
	m.ProcessDir(REALM_CONFIG, func(st *np.Stat) (bool, error) {
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
		nodedOverprovisioned := false
		for _, nd := range rCfg.NodedsAssigned {
			if m.nodedOverprovisioned(realmId, nd) {
				nodedOverprovisioned = true
				break
			}
		}
		// If there are more than the minimum number of required Nodeds available...
		if len(rCfg.NodedsAssigned) > nReplicas() && nodedOverprovisioned {
			opRealmId = realmId
			ok = true
			return true, nil
		}
		return false, nil
	})
	return opRealmId, ok
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
func (m *SigmaResourceMgr) requestCores(realmId string) {
	db.DPrintf("SIGMAMGR", "Sigmamgr requesting cores from %v", realmId)
	msg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, "", 1)
	resource.SendMsg(m.FsLib, path.Join(realmMgrPath(realmId), np.RESOURCE_CTL), msg)
}

// Destroy a realm.
func (m *SigmaResourceMgr) destroyRealm(realmId string) {
	m.Lock()
	defer m.Unlock()

	db.DPrintf("SIGMAMGR", "Destroy realm %v", realmId)

	lockRealm(m.realmLocks[realmId], realmId)

	// Update the realm config to note that the realm is being shut down.
	cfg := &RealmConfig{}
	m.ReadConfig(RealmConfPath(realmId), cfg)
	cfg.Shutdown = true
	m.WriteConfig(RealmConfPath(realmId), cfg)

	unlockRealm(m.realmLocks[realmId], realmId)
	delete(m.realmLocks, realmId)

	// Send a message to the realmmmgr telling it to kill its realm.
	msg := resource.MakeResourceMsg(resource.Trequest, resource.Trealm, "", 1)
	resource.SendMsg(m.FsLib, path.Join(realmMgrPath(realmId), np.RESOURCE_CTL), msg)

	m.evictRealmMgr(realmId)
	db.DPrintf("SIGMAMGR", "Done destroying realm %v", realmId)
}

func (m *SigmaResourceMgr) startRealmMgr(realmId string) {
	pid := proc.Tpid("realmmgr-" + proc.GenPid().String())
	p := proc.MakeProcPid(pid, "realm/realmmgr", []string{realmId})
	if _, err := m.SpawnKernelProc(p, fslib.Named()); err != nil {
		db.DFatalf("Error spawn realmmgr %v", err)
	}
	if err := m.WaitStart(p.Pid); err != nil {
		db.DFatalf("Error WaitStart realmmgr %v", err)
	}
	db.DPrintf("SIGMAMGR", "Sigmamgr started realmmgr %v in realm %v", pid.String(), realmId)
	m.realmmgrs[realmId] = pid
}

func (m *SigmaResourceMgr) evictRealmMgr(realmId string) {
	pid := m.realmmgrs[realmId]
	db.DPrintf("SIGMAMGR", "Sigmamgr evicting realmmgr %v in realm %v", pid.String(), realmId)
	if err := m.Evict(pid); err != nil {
		db.DFatalf("Error evict realmmgr %v for realm %v", pid, realmId)
	}
	if status, err := m.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
		db.DFatalf("Error bad status evict realmmgr %v for realm %v: status %v err %v", pid, realmId, status, err)
	}
	delete(m.realmmgrs, realmId)
}

func (m *SigmaResourceMgr) Work() {
	m.Serve()
	m.Done()
}
