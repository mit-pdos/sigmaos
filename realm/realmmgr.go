package realm

import (
	"encoding/json"
	"log"
	"os/exec"
	"path"
	"strings"
	"time"

	"ulambda/config"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/sync"
)

const (
	SCAN_INTERVAL_MS = 10
)

const (
	NO_REALM = "no-realm"
)

const (
	FREE_REALMDS  = "name/free-realmds"  // Unassigned realmds
	REALM_CREATE  = "name/realm-create"  // Realm allocation requests
	REALM_DESTROY = "name/realm-destroy" // Realm destruction requests
	REALMS        = "name/realms"        // List of realms, with realmds registered under them
	REALM_CONFIG  = "name/realm-config"  // Store of realm configs
	REALMD_CONFIG = "name/realmd-config" // Store of realmd configs
	REALM_NAMEDS  = "name/realm-nameds"  // Symlinks to realms' nameds
)

type RealmMgr struct {
	nameds       []*exec.Cmd
	freeRealmds  *sync.FilePriorityBag
	realmCreate  *sync.FilePriorityBag
	realmDestroy *sync.FilePriorityBag
	done         chan bool
	*config.ConfigClnt
	*fslib.FsLib
}

func MakeRealmMgr(bin string) *RealmMgr {
	m := &RealmMgr{}
	m.done = make(chan bool)
	nameds, err := BootNamedReplicas(nil, bin, fslib.Named(), NO_REALM)
	m.nameds = nameds
	// Start a named instance.
	if err != nil {
		log.Fatalf("Error BootNamed in MakeRealmMgr: %v", err)
	}
	m.FsLib = fslib.MakeFsLib("realmmgr")
	m.ConfigClnt = config.MakeConfigClnt(m.FsLib)
	m.makeInitFs()
	m.makeFileBags()

	return m
}

func (m *RealmMgr) makeInitFs() {
	if err := m.Mkdir(REALMS, 0777); err != nil {
		log.Fatalf("Error Mkdir REALMS in RealmMgr.makeInitFs: %v", err)
	}
	if err := m.Mkdir(REALM_CONFIG, 0777); err != nil {
		log.Fatalf("Error Mkdir REALM_CONFIG in RealmMgr.makeInitFs: %v", err)
	}
	if err := m.Mkdir(REALMD_CONFIG, 0777); err != nil {
		log.Fatalf("Error Mkdir REALMD_CONFIG in RealmMgr.makeInitFs: %v", err)
	}
	if err := m.Mkdir(REALM_NAMEDS, 0777); err != nil {
		log.Fatalf("Error Mkdir REALM_NAMEDS in RealmMgr.makeInitFs: %v", err)
	}
}

func (m *RealmMgr) makeFileBags() {
	// Set up FilePriorityBags
	m.freeRealmds = sync.MakeFilePriorityBag(m.FsLib, FREE_REALMDS)
	m.realmCreate = sync.MakeFilePriorityBag(m.FsLib, REALM_CREATE)
	m.realmDestroy = sync.MakeFilePriorityBag(m.FsLib, REALM_DESTROY)
}

// Handle realm creation requests.
func (m *RealmMgr) createRealms() {
	for {
		// Get a realm creation request
		_, realmId, b, err := m.realmCreate.Get()
		if err != nil {
			log.Fatalf("Error Get in RealmMgr.createRealms: %v", err)
		}

		realmLock := sync.MakeLock(m.FsLib, named.LOCKS, REALM_LOCK+realmId, true)
		realmLock.Lock()

		// Unmarshal the realm config file.
		cfg := &RealmConfig{}
		if err := json.Unmarshal(b, cfg); err != nil {
			log.Fatalf("Error Unmarshal in RealmMgr.createRealms: %v", err)
		}

		// Make a directory for this realm.
		if err := m.Mkdir(path.Join(REALMS, realmId), 0777); err != nil {
			log.Fatalf("Error Mkdir in RealmMgr.createRealms: %v", err)
		}

		// Make a directory for this realm's nameds.
		if err := m.Mkdir(path.Join(REALM_NAMEDS, realmId), 0777); err != nil {
			log.Fatalf("Error Mkdir in RealmMgr.createRealms: %v", err)
		}

		// Make the realm config file.
		m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

		realmLock.Unlock()
	}
}

// Deallocate a realmd from a realm.
func (m *RealmMgr) deallocRealmd(realmId string, realmdId string) {
	rdCfg := &RealmdConfig{}
	rdCfg.Id = realmdId
	rdCfg.RealmId = NO_REALM
	// Update the realmd config file.
	m.WriteConfig(path.Join(REALMD_CONFIG, realmdId), rdCfg)

	// Note realmd de-registration
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
	rCfg.NRealmds -= 1
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
}

func (m *RealmMgr) deallocAllRealmds(realmId string) {
	rds, err := m.ReadDir(path.Join(REALMS, realmId))
	if err != nil {
		log.Fatalf("Error ReadDir in RealmMgr.deallocRealms: %v", err)
	}

	for _, realmd := range rds {
		m.deallocRealmd(realmId, realmd.Name)
	}
}

func (m *RealmMgr) destroyRealms() {
	for {
		// Get a realm creation request
		_, realmId, _, err := m.realmDestroy.Get()
		if err != nil {
			log.Fatalf("Error Get in RealmMgr.destroyRealms: %v", err)
		}

		realmLock := sync.MakeLock(m.FsLib, named.LOCKS, REALM_LOCK+realmId, true)
		realmLock.Lock()

		m.deallocAllRealmds(realmId)

		cfg := &RealmConfig{}
		m.ReadConfig(path.Join(REALM_CONFIG, realmId), cfg)
		cfg.Shutdown = true
		m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

		realmLock.Unlock()
	}
}

// Get & alloc a realmd to this realm.
func (m *RealmMgr) allocRealmd(realmId string) {
	// Get a free realmd
	_, realmdId, _, err := m.freeRealmds.Get()
	if err != nil {
		log.Fatalf("Error Get in RealmMgr.allocRealmd: %v", err)
	}

	// Update the realmd's config
	rdCfg := &RealmdConfig{}
	rdCfg.Id = realmdId
	rdCfg.RealmId = realmId
	m.WriteConfig(path.Join(REALMD_CONFIG, realmdId), rdCfg)

	// Update the realm's config
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
	rCfg.NRealmds += 1
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
}

func (m *RealmMgr) needsRealmd(realmId string) bool {
	// If the realm is being shut down, the realm config file may not be there
	// anymore. In this case, another realmd is not needed.
	if _, err := m.Stat(path.Join(REALM_CONFIG, realmId)); err != nil && strings.Contains(err.Error(), "file not found") {
		return false
	}
	cfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), cfg)
	// If we are below the target replication level, start a new realmd
	if cfg.NRealmds < nReplicas() && !cfg.Shutdown {
		return true
	}
	// TODO: scan utilization
	return false
}

// Balance realmds across realms.
func (m *RealmMgr) balanceRealmds() {
	for {
		realms, err := m.ReadDir(REALMS)
		if err != nil {
			log.Fatalf("Error ReadDir in RealmMgr.balanceRealmds: %v", err)
		}

		for _, realm := range realms {
			realmId := realm.Name
			// XXX Currently we assume there are always enough realmds for the number
			// of realms we have. If that assumption is broken, this may deadlock when
			// a realm is trying to exit & we're trying to assign a realmd to it.
			realmLock := sync.MakeLock(m.FsLib, named.LOCKS, REALM_LOCK+realmId, true)
			realmLock.Lock()

			if m.needsRealmd(realmId) {
				m.allocRealmd(realmId)
			}

			realmLock.Unlock()
		}

		time.Sleep(SCAN_INTERVAL_MS * time.Millisecond)
	}
}

func (m *RealmMgr) Work() {
	go m.createRealms()
	go m.destroyRealms()
	go m.balanceRealmds()
	<-m.done
}
