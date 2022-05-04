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
)

const (
	sigmactl     = "sigmactl"
	SIGMACTL     = np.SIGMA_MGR + "/" + sigmactl // SigmaResourceMgr control file.
	REALM_CONFIG = "name/realm-config"           // Store of realm configs
	NODED_CONFIG = "name/noded-config"           // Store of noded configs
	REALM_NAMEDS = "name/realm-nameds"           // Symlinks to realms' nameds
	REALM_FENCES = "name/realm-fences"           // Fence around modifications to realm allocations.
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
	m.MemFs, m.FsLib, _, err = fslibsrv.MakeMemFs(np.SIGMA_MGR, "sigmamgr")
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
	if err := m.MkDir(REALM_CONFIG, 0777); err != nil {
		db.DFatalf("Error Mkdir REALM_CONFIG in SigmaResourceMgr.makeInitFs: %v", err)
	}
	if err := m.MkDir(NODED_CONFIG, 0777); err != nil {
		db.DFatalf("Error Mkdir NODED_CONFIG in SigmaResourceMgr.makeInitFs: %v", err)
	}
	if err := m.MkDir(REALM_NAMEDS, 0777); err != nil {
		db.DFatalf("Error Mkdir REALM_NAMEDS in SigmaResourceMgr.makeInitFs: %v", err)
	}
	if err := m.MkDir(REALM_FENCES, 0777); err != nil {
		db.DFatalf("Error Mkdir REALM_FENCES in SigmaResourceMgr.makeInitFs: %v", err)
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

func (m *SigmaResourceMgr) receiveResourceGrant(msg *ResourceMsg) {
	switch msg.ResourceType {
	case Trealm:
		m.destroyRealm(msg.Name)
	case Tnode:
		db.DPrintf(db.ALWAYS, "free noded %v", msg.Name)
		m.freeNodeds <- msg.Name
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (m *SigmaResourceMgr) handleResourceRequest(msg *ResourceMsg) {
	switch msg.ResourceType {
	case Trealm:
		m.createRealm(msg.Name)
	case Tnode:
		m.growRealm(msg.Name)
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (m *SigmaResourceMgr) getFreeNoded(nRetries int) string {
	for i := 0; i < nRetries; i++ {
		select {
		case nodedId := <-m.freeNodeds:
			return nodedId
		default:
			db.DPrintf(db.ALWAYS, "Tried to get Noded, but none free.")
			time.Sleep(10 * time.Millisecond)
		}
	}
	return ""
}

// Alloc a Noded to this realm.
func (m *SigmaResourceMgr) allocNoded(realmId string, nodedId string) {
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
		nodedId := m.getFreeNoded(100)
		if nodedId == "" {
			db.DFatalf("No free noded available")
		}
		m.allocNoded(realmId, nodedId)
	}
}

func (m *SigmaResourceMgr) growRealm(realmId string) {
	nodedId := m.getFreeNoded(100)
	if nodedId == "" {
		db.DPrintf("SIGMAMGR", "Sigmamgr couldn't grow realm %v", realmId)
		return
	}
	m.allocNoded(realmId, nodedId)
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
	defer unlockRealm(m.ecs[realmId], realmId)

	cfg := &RealmConfig{}
	cfg.Rid = realmId

	// Make the realm config file.
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

	// Allocate the minimum number of Nodeds required to start this realm. For
	// now, this is nReplicas() for all realms.
	m.allocMinNodeds(realmId)
}

// Request a Noded from realm realmId.
func (m *SigmaResourceMgr) requestNoded(realmId string) {
	msg := MakeResourceMsg(Trequest, Tnode, "", 1)
	for {
		// TODO: move realmctl file to sigma named.
		if _, err := m.SetFile(path.Join(REALM_NAMEDS, realmId, "realmmgr", realmctl), msg.Marshal(), np.OWRITE, 0); err != nil {
			db.DPrintf(db.ALWAYS, "Error SetFile in SigmaResourceMgr.requestNoded: %v", err)
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
}

func (m *SigmaResourceMgr) Work() {
	m.Serve()
	m.Done()
}
