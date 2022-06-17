package realm

import (
	"path"
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
)

const (
	REALM_CONFIG = "name/realm-config" // Store of realm configs
	NODED_CONFIG = "name/noded-config" // Store of noded configs
	REALM_NAMEDS = "name/realm-nameds" // Symlinks to realms' nameds
	REALM_FENCES = "name/realm-fences" // Fence around modifications to realm allocations.
	REALM_MGRS   = "name/realm-mgrs"   // Fence around modifications to realm allocations.
)

type SigmaResourceMgr struct {
	sync.Mutex
	freeCoreGroups int64
	realmCreate    chan string
	realmDestroy   chan string
	realmmgrs      map[string]proc.Tpid
	ecs            map[string]*electclnt.ElectClnt
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
	m.makeInitFs()
	resource.MakeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, m.Root(), np.RESOURCE_CTL)
	m.ecs = make(map[string]*electclnt.ElectClnt)
	m.realmmgrs = make(map[string]proc.Tpid)

	return m
}

// Make the initial realm dirs, and remove the unneeded union dirs.
func (m *SigmaResourceMgr) makeInitFs() {
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
			db.DFatalf("Error Mkdir %v in SigmaResourceMgr.makeInitFs: %v", d, err)
		}
	}
}

func (m *SigmaResourceMgr) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Trealm:
		m.destroyRealm(msg.Name)
	case resource.Tnode:
		db.DPrintf("SIGMAMGR", "free noded %v", msg.Name)
		db.DPrintf(db.ALWAYS, "free noded %v", msg.Name)
	case resource.Tcore:
		m.freeCores(1)
		db.DPrintf(db.ALWAYS, "free cores %v", msg.Name)
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

		m.growRealmL(msg.Name)
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// TODO: should probably release lock in this loop.
func (m *SigmaResourceMgr) tryGetFreeCores(nRetries int) bool {
	for i := 0; i < nRetries; i++ {
		if atomic.LoadInt64(&m.freeCoreGroups) > 1 {
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
	if _, err := m.SetFile(path.Join(REALM_MGRS, realmId, np.RESOURCE_CTL), msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile: %v", err)
	}
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
	m.requestNoded(opRealmId)
	// Wait for the over-provisioned realm to cede its cores.
	if m.tryGetFreeCores(100) {
		// Allocate cores to this realm.
		m.allocCores(realmId, 1)
		return true
	}
	return false
}

// Find an over-provisioned realm (a realm with resources to spare). Returns
// true if an overprovisioned realm was found, false otherwise.
//
// TODO: determine overprovisioned status by resource utilization.
func (m *SigmaResourceMgr) findOverProvisionedRealm(ignoreRealm string) (string, bool) {
	opRealmId := ""
	ok := false
	// XXX Eventually, we'll want to find overprovisioned realms according to
	// more nuanced metrics, e.g. how many Nodeds are running BE vs LC tasks, how
	// many Nodeds are running procs that hold state, etc.
	m.ProcessDir(REALM_CONFIG, func(st *np.Stat) (bool, error) {
		realmId := st.Name

		// Avoid thrashing.
		if realmId == ignoreRealm {
			return false, nil
		}

		lockRealm(m.ecs[realmId], realmId)
		defer unlockRealm(m.ecs[realmId], realmId)

		rCfg := &RealmConfig{}
		m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)

		// If there are more than the minimum number of required Nodeds available...
		if len(rCfg.NodedsAssigned) > nReplicas() {
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
	if _, ok := m.ecs[realmId]; ok {
		db.DFatalf("tried to create realm twice %v", realmId)
	}
	m.ecs[realmId] = electclnt.MakeElectClnt(m.FsLib, path.Join(REALM_FENCES, realmId), 0777)

	lockRealm(m.ecs[realmId], realmId)

	cfg := &RealmConfig{}
	cfg.Rid = realmId

	// Make the realm config file.
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

	unlockRealm(m.ecs[realmId], realmId)

	// Start this realm's realmmgr.
	m.startRealmMgr(realmId)
}

// Request a Noded from realm realmId.
func (m *SigmaResourceMgr) requestNoded(realmId string) {
	msg := resource.MakeResourceMsg(resource.Trequest, resource.Tnode, "", 1)
	if _, err := m.SetFile(path.Join(REALM_MGRS, realmId, np.RESOURCE_CTL), msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile: %v", err)
	}
}

// Destroy a realm.
func (m *SigmaResourceMgr) destroyRealm(realmId string) {
	m.Lock()
	defer m.Unlock()

	db.DPrintf("SIGMAMGR", "Destroy realm %v", realmId)

	lockRealm(m.ecs[realmId], realmId)

	// Update the realm config to note that the realm is being shut down.
	cfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), cfg)
	cfg.Shutdown = true
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

	unlockRealm(m.ecs[realmId], realmId)
	delete(m.ecs, realmId)

	// Request all Nodeds from the realm. This has to happen without the
	// protection of the realm lock, because the realm lock must be held by the
	// RealmMgr in order to proceed with deallocation.
	for _ = range cfg.NodedsAssigned {
		m.requestNoded(realmId)
	}
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
