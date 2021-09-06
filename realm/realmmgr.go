package realm

import (
	"encoding/json"
	"log"
	"math/rand"
	"path"

	"ulambda/atomic"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/sync"
)

const (
	NO_REALM = "no-realm"
)

const (
	FREE_REALMDS  = "name/free-realmds"  // Unassigned realmds
	REALM_ALLOC   = "name/realm-alloc"   // Realm allocation requests
	REALMS        = "name/realms"        // List of realms, with realmds registered under them
	REALM_CONFIG  = "name/realm-config"  // Store of realm configs
	REALMD_CONFIG = "name/realmd-config" // Store of realmd configs
)

type RealmMgr struct {
	s           *kernel.System
	freeRealmds *sync.FilePriorityBag
	realmAlloc  *sync.FilePriorityBag
	done        chan bool
	*fslib.FsLib
}

func MakeRealmMgr() *RealmMgr {
	m := &RealmMgr{}
	m.done = make(chan bool)
	m.s = kernel.MakeSystem("..")
	// Start a named instance.
	m.s.BootMin()
	m.FsLib = fslib.MakeFsLib("realmmgr")
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
	m.realmAlloc = sync.MakeFilePriorityBag(m.FsLib, REALM_ALLOC)
}

// Handle realm allocation requests.
func (m *RealmMgr) allocRealms() {
	for {
		// Get a realm allocation request
		_, rid, b, err := m.realmAlloc.Get()
		if err != nil {
			log.Fatalf("Error Get in RealmMgr.allocRealms: %v", err)
		}
		// Make a directory for this realm.
		if err := m.Mkdir(path.Join(REALMS, rid), 0777); err != nil {
			log.Fatalf("Error Mkdir in RealmMgr.allocRealms: %v", err)
		}
		// Make the realm config file.
		if err := atomic.MakeFileAtomic(m.FsLib, path.Join(REALM_CONFIG, rid), 0777, b); err != nil {
			log.Fatalf("Error MakeFileAtomic in RealmMgr.allocRealms: %v", err)
		}
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
func (m *RealmMgr) assignRealmd(realmdId string, rid string) {
	fpath := path.Join(REALMD_CONFIG, realmdId)
	b, _, err := m.GetFile(fpath)
	if err != nil {
		log.Fatalf("Error GetFile in RealmMgr.assignRealmd: %v", err)
	}
	realmd := &RealmdConfig{}
	err = json.Unmarshal(b, realmd)
	if err != nil {
		log.Fatalf("Error Unmarshal in RealmMgr.assignRealmd: %v", err)
	}
	realmd.RealmId = rid
	// Update the realm config file.
	if err := atomic.MakeFileJsonAtomic(m.FsLib, fpath, 0777, realmd); err != nil {
		log.Fatalf("Error MakeFileAtomic in RealmMgr.allocRealms: %v", err)
	}
}

// Assign free realmds to realms.
func (m *RealmMgr) assignRealmds() {
	for {
		rPriority, realmd, b, err := m.freeRealmds.Get()
		if err != nil {
			log.Fatalf("Error Get in RealmMgr.assignRealmds: %v", err)
		}
		rid := m.selectRealm()
		// If there are no realms to assign this realmd to, try again later.
		if rid == NO_REALM {
			// TODO: Avoid spinning when no realms are available.
			if err := m.freeRealmds.Put(rPriority, realmd, b); err != nil {
				log.Fatalf("Error Put in RealmMgr.assignRealmds: %v", err)
			}
			continue
		}
		m.assignRealmd(realmd, rid)
	}
}

func (m *RealmMgr) Work() {
	go m.allocRealms()
	go m.assignRealmds()
	<-m.done
}

// TODO: unassign/reassign realmds
