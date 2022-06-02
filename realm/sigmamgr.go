package realm

import (
	"path"
	"sync"
	"time"

	"ulambda/config"
	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/electclnt"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	np "ulambda/ninep"
	"ulambda/resource"
)

const (
	sigmactl     = "sigmactl"
	SIGMACTL     = np.SIGMAMGR + "/" + sigmactl // SigmaResourceMgr control file.
	REALM_CONFIG = "name/realm-config"          // Store of realm configs
	NODED_CONFIG = "name/noded-config"          // Store of noded configs
	REALM_NAMEDS = "name/realm-nameds"          // Symlinks to realms' nameds
	REALM_FENCES = "name/realm-fences"          // Fence around modifications to realm allocations.
	REALM_MGRS   = "name/realm-mgrs"            // Fence around modifications to realm allocations.
)

type SigmaResourceMgr struct {
	sync.Mutex
	freeNodeds   chan string
	realmCreate  chan string
	realmDestroy chan string
	root         fs.Dir
	ecs          map[string]*electclnt.ElectClnt
	*config.ConfigClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func MakeSigmaResourceMgr() *SigmaResourceMgr {
	m := &SigmaResourceMgr{}
	m.freeNodeds = make(chan string)
	m.realmCreate = make(chan string)
	m.realmDestroy = make(chan string)
	var err error
	m.MemFs, m.FsLib, _, err = fslibsrv.MakeMemFs(np.SIGMAMGR, "sigmamgr")
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}
	m.ConfigClnt = config.MakeConfigClnt(m.FsLib)
	m.makeInitFs()
	m.makeCtlFiles()
	m.ecs = make(map[string]*electclnt.ElectClnt)

	return m
}

// Make the initial realm dirs, and remove the unneeded union dirs.
func (m *SigmaResourceMgr) makeInitFs() {
	dirs := []string{
		REALM_CONFIG,
		NODED_CONFIG,
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

func (m *SigmaResourceMgr) makeCtlFiles() {
	// Set up control files
	ctl := makeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, nil, m.Root())
	err := dir.MkNod(ctx.MkCtx("", 0, nil), m.Root(), sigmactl, ctl)
	if err != nil {
		db.DFatalf("Error MkNod sigmactl: %v", err)
	}
}

func (m *SigmaResourceMgr) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Trealm:
		m.destroyRealm(msg.Name)
	case resource.Tnode:
		db.DPrintf("SIGMAMGR", "free noded %v", msg.Name)
		m.freeNodeds <- msg.Name
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// Handle a resource request.
func (m *SigmaResourceMgr) handleResourceRequest(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Trealm:
		m.createRealm(msg.Name)
	case resource.Tnode:
		m.Lock()
		defer m.Unlock()

		m.growRealm(msg.Name)
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (m *SigmaResourceMgr) getFreeNoded(nRetries int) (string, bool) {
	for i := 0; i < nRetries; i++ {
		select {
		case nodedId := <-m.freeNodeds:
			return nodedId, true
		default:
			db.DPrintf("SIGMAMGR", "Tried to get Noded, but none free.")
			time.Sleep(10 * time.Millisecond)
		}
	}
	return "", false
}

// Alloc a Noded to this realm.
func (m *SigmaResourceMgr) allocNoded(realmId string, nodedId string) {
	// If the realm has been destroyed, exit early.
	if _, ok := m.ecs[realmId]; !ok {
		return
	}

	lockRealm(m.ecs[realmId], realmId)
	defer unlockRealm(m.ecs[realmId], realmId)

	// Update the noded's config
	rdCfg := &NodedConfig{}
	rdCfg.Id = nodedId
	rdCfg.RealmId = realmId
	m.WriteConfig(path.Join(NODED_CONFIG, nodedId), rdCfg)

	// Update the realm's config
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
	rCfg.NodedsAssigned = append(rCfg.NodedsAssigned, nodedId)
	rCfg.LastResize = time.Now()
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
}

// Allocate the minimum number of Nodeds needed to start a realm.
func (m *SigmaResourceMgr) allocMinNodeds(realmId string) {
	n := nReplicas()
	for i := 0; i < n; i++ {
		// Retry noded allocation infinitely for now.
		if ok := m.growRealm(realmId); !ok {
			db.DFatalf("Can't allocate min nodeds for realm %v", realmId)
		}
	}
}

// Tries to add a Noded to a realm. Will first try and pull from the list of
// free Nodeds, and if none is available, it will try to make one free, and
// then retry.
func (m *SigmaResourceMgr) growRealm(realmId string) bool {
	// Try to get a free noded.
	if nodedId, ok := m.getFreeNoded(100); ok {
		// Alloc the free Noded.
		m.allocNoded(realmId, nodedId)
		return true
	}
	// No noded was available, so try to find a realm with spare resources.
	opRealmId, ok := m.findOverProvisionedRealm(realmId)
	if !ok {
		db.DPrintf("SIGMAMGR", "No overprovisioned realms available")
		return false
	}
	// Ask the over-provisioned realm to give up a Noded.
	m.requestNoded(opRealmId)
	// Try to get the newly freed Noded.
	if nodedId, ok := m.getFreeNoded(100); ok {
		// Alloc the newly freed Noded.
		m.allocNoded(realmId, nodedId)
		return true
	}
	return false
}

// Find an over-provisioned realm (a realm with resources to spare). Returns
// true if an overprovisioned realm was found, false otherwise.
func (m *SigmaResourceMgr) findOverProvisionedRealm(ignoreRealm string) (string, bool) {
	opRealmId := ""
	success := false
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
			success = true
			return true, nil
		}
		return false, nil
	})
	return opRealmId, success
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

	// Allocate the minimum number of Nodeds required to start this realm. For
	// now, this is nReplicas() for all realms.
	m.allocMinNodeds(realmId)
}

// Request a Noded from realm realmId.
func (m *SigmaResourceMgr) requestNoded(realmId string) {
	msg := resource.MakeResourceMsg(resource.Trequest, resource.Tnode, "", 1)
	for {
		// TODO: move realmctl file to sigma named.
		if _, err := m.SetFile(path.Join(REALM_MGRS, realmId, realmctl), msg.Marshal(), np.OWRITE, 0); err != nil {
			db.DPrintf("SIGMAMGR_ERR", "Error SetFile in SigmaResourceMgr.requestNoded: %v", err)
		} else {
			return
		}
		time.Sleep(10 * time.Millisecond)
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
	db.DPrintf("SIGMAMGR", "Done destroying realm %v", realmId)
}

func (m *SigmaResourceMgr) Work() {
	m.Serve()
	m.Done()
}
