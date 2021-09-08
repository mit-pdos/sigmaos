package realm

import (
	"encoding/json"
	"log"
	"math/rand"
	"os/exec"
	"path"

	"ulambda/config"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/sync"
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
)

type RealmMgr struct {
	named        *exec.Cmd
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
	named, err := BootNamed(bin, fslib.Named())
	m.named = named
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
		_, rid, b, err := m.realmCreate.Get()
		if err != nil {
			log.Fatalf("Error Get in RealmMgr.createRealms: %v", err)
		}

		// Unmarshal the realm config file.
		cfg := &RealmConfig{}
		if err := json.Unmarshal(b, cfg); err != nil {
			log.Fatalf("Error Unmarshal in RealmMgr.createRealms: %v", err)
		}

		// Make a directory for this realm.
		if err := m.Mkdir(path.Join(REALMS, rid), 0777); err != nil {
			log.Fatalf("Error Mkdir in RealmMgr.createRealms: %v", err)
		}

		// Make the realm config file.
		m.WriteConfig(path.Join(REALM_CONFIG, rid), cfg)
	}
}

// Deallocate a realmd from a realm.
func (m *RealmMgr) deallocRealmd(realmdId string) {
	cfg := &RealmdConfig{}
	cfg.Id = realmdId
	cfg.RealmId = NO_REALM
	// Update the realm config file.
	m.WriteConfig(path.Join(REALMD_CONFIG, realmdId), cfg)
}

func (m *RealmMgr) deallocRealmds(rid string) {
	realmLock := sync.MakeLock(m.FsLib, named.LOCKS, REALM_LOCK+rid, true)

	realmLock.Lock()
	defer realmLock.Unlock()

	rds, err := m.ReadDir(path.Join(REALMS, rid))
	if err != nil {
		log.Fatalf("Error ReadDir in RealmMgr.deallocRealms: %v", err)
	}

	for _, rd := range rds {
		m.deallocRealmd(rd.Name)
	}
}

func (m *RealmMgr) destroyRealms() {
	for {
		// Get a realm creation request
		_, rid, _, err := m.realmDestroy.Get()
		if err != nil {
			log.Fatalf("Error Get in RealmMgr.createRealms: %v", err)
		}
		m.deallocRealmds(rid)
	}
}

// Select a realm to assign a new realmd to. Currently done by random choice.
func (m *RealmMgr) selectRealm() string {
	realms, err := m.ReadDir(REALMS)
	if err != nil {
		log.Fatalf("Error ReadDir in RealmMgr.selectRealm: %v", err)
	}
	if len(realms) == 0 {
		return NO_REALM
	}
	choice := rand.Intn(len(realms))
	return realms[choice].Name
}

// Set the realm id in the realmd's config file & trigger its watch.
func (m *RealmMgr) allocRealmd(realmdId string, realmId string) {
	cfg := &RealmdConfig{}
	cfg.Id = realmdId
	cfg.RealmId = realmId
	m.WriteConfig(path.Join(REALMD_CONFIG, realmdId), cfg)
}

// Assign free realmds to realms.
func (m *RealmMgr) allocRealmds() {
	for {
		rPriority, realmd, b, err := m.freeRealmds.Get()
		if err != nil {
			log.Fatalf("Error Get in RealmMgr.allocRealmds: %v", err)
		}
		rid := m.selectRealm()
		// If there are no realms to assign this realmd to, try again later.
		if rid == NO_REALM {
			// TODO: Avoid spinning when no realms are available.
			if err := m.freeRealmds.Put(rPriority, realmd, b); err != nil {
				log.Fatalf("Error Put in RealmMgr.allocRealmds: %v", err)
			}
			continue
		}
		m.allocRealmd(realmd, rid)
	}
}

func (m *RealmMgr) Work() {
	go m.createRealms()
	go m.destroyRealms()
	go m.allocRealmds()
	<-m.done
}
