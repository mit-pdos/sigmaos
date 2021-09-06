package realm

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/atomic"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/named"
	"ulambda/sync"
)

const (
	DEFAULT_REALMD_PRIORITY = "0"
	REALM_LOCK              = "realm-lock."
)

type RealmdConfig struct {
	Id      string
	RealmId string
}

type Realmd struct {
	id          string
	cfg         *RealmdConfig
	s           *kernel.System
	freeRealmds *sync.FilePriorityBag
	realmLock   *sync.Lock
	*fslib.FsLib
}

func MakeRealmd() *Realmd {
	// XXX Get id somehow
	id, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("Error LocalIP in MakeRealmd: %v", err)
	}
	r := &Realmd{}
	r.id = id
	r.FsLib = fslib.MakeFsLib(fmt.Sprintf("realmd-%v", id))

	// Set up the realmd config
	r.cfg = &RealmdConfig{}
	r.cfg.Id = id
	r.cfg.RealmId = NO_REALM

	b, err := json.Marshal(r.cfg)
	if err != nil {
		log.Fatalf("Error Marshal in MakeRealm: %v", err)
	}

	r.freeRealmds = sync.MakeFilePriorityBag(r.FsLib, FREE_REALMDS)

	if err := r.freeRealmds.Put(DEFAULT_REALMD_PRIORITY, id, b); err != nil {
		log.Fatalf("Error Put in MakeRealmd: %v", err)
	}

	return r
}

func (r *Realmd) watchConfig(done chan bool) {
	fpath := path.Join(REALMD_CONFIG, r.id)
	err := r.SetRemoveWatch(fpath, func(path string, err error) {
		if err != nil {
			log.Fatalf("Error Watch in Realmd.watchConfig: %v", err)
		}
		done <- true
	})
	if err != nil {
		log.Fatalf("Error SetRemoveWatch in Realmd.watchConfig: %v", err)
	}
}

// Update configuration.
func (r *Realmd) readConfig() {
	for {
		fpath := path.Join(REALMD_CONFIG, r.id)
		err := r.ReadFileJson(fpath, r.cfg)
		if err == nil && r.cfg.RealmId != NO_REALM {
			break
		}
		done := make(chan bool)
		go r.watchConfig(done)
		<-done
	}
	r.realmLock = sync.MakeLock(r.FsLib, named.LOCKS, REALM_LOCK+r.id, true)
}

// If this is the first realmd assigned to a realm, initialize the realm by
// starting a named for it.
func (r *Realmd) tryInitRealm() {
	rds, err := r.ReadDir(path.Join(REALMS, r.cfg.RealmId))
	if err != nil {
		log.Fatalf("Error ReadDir in Realmd.tryInitRealm: %v", err)
	}

	// If this is the first realmd, start the realm's named.
	if len(rds) == 0 {
		ip, err := fsclnt.LocalIP()
		if err != nil {
			log.Fatalf("Error LocalIP in Realmd.tryInitRealm: %v", err)
		}
		namedAddr := genNamedAddr(ip)
		s := kernel.MakeSystem("..")

		// Start a named instance.
		s.BootNamed(namedAddr)

		realmCfg := getRealmConfig(r.FsLib, r.cfg.RealmId)
		realmCfg.NamedAddr = namedAddr
		setRealmConfig(r.FsLib, realmCfg)

		// Signal that the realm has been initialized
		rStartCond := sync.MakeCond(r.FsLib, path.Join(kernel.BOOT, r.cfg.RealmId), nil)
		rStartCond.Destroy()
	}
}

// Register this realmd as part of a realm.
func (r *Realmd) registerL() {
	// Register this realmd as belonging to this realm.
	if err := atomic.MakeFileAtomic(r.FsLib, path.Join(REALMS, r.cfg.RealmId), 0777, []byte{}); err != nil {
		log.Fatalf("Error MakeFileAtomic in Realmd.register: %v", err)
	}
}

func (r *Realmd) boot(realmCfg *RealmConfig) {
	// TODO: boot
	r.s = kernel.MakeSystemNamedAddr("..", realmCfg.NamedAddr)
	r.s.BootFollower()
}

func (r *Realmd) Work() {
	for {
		// Get the next realm assignment.
		r.readConfig()

		r.realmLock.Lock()

		// Try to initalize this realm.
		r.tryInitRealm()
		// Get the realm config
		realmCfg := getRealmConfig(r.FsLib, r.cfg.RealmId)
		// Register this realmd
		r.registerL()
		// Boot this realmd's system services
		r.boot(realmCfg)
		// Watch for changes to the config
		done := make(chan bool)
		go r.watchConfig(done)

		r.realmLock.Unlock()

		// Wait for the watch to trigger
		<-done
	}
}
