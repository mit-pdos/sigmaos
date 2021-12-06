package realm

import (
	"fmt"
	"log"
	"path"

	"ulambda/atomic"
	"ulambda/config"
	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/named"
	"ulambda/procclnt"
	"ulambda/sync"
)

const (
	DEFAULT_MACHINED_PRIORITY = "0"
	REALM_LOCK                = "realm-lock."
)

type MachinedConfig struct {
	Id      string
	RealmId string
}

type Machined struct {
	id        string
	bin       string
	cfgPath   string
	cfg       *MachinedConfig
	s         *kernel.System
	realmLock *sync.Lock
	*config.ConfigClnt
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MakeMachined(bin string, id string) *Machined {
	// XXX Get id somehow
	r := &Machined{}
	r.id = id
	r.bin = bin
	r.cfgPath = path.Join(MACHINED_CONFIG, id)
	r.FsLib = fslib.MakeFsLib(fmt.Sprintf("machined-%v", id))
	r.ProcClnt = procclnt.MakeProcClntInit(r.FsLib, fslib.Named())
	r.ConfigClnt = config.MakeConfigClnt(r.FsLib)

	// Set up the machined config
	r.cfg = &MachinedConfig{}
	r.cfg.Id = id
	r.cfg.RealmId = kernel.NO_REALM

	// Write the initial config file
	r.WriteConfig(r.cfgPath, r.cfg)

	// Mark self as available for allocation
	r.markFree()

	return r
}

// Mark self as available for allocation to a realm.
func (r *Machined) markFree() {
	cfg := &MachinedConfig{}
	cfg.Id = r.id
	cfg.RealmId = kernel.NO_REALM

	if err := r.WriteFile(FREE_MACHINEDS, []byte(r.id)); err != nil {
		log.Fatalf("Error WriteFile in MakeMachined: %v %v", FREE_MACHINEDS, err)
	}
}

// Update configuration.
func (r *Machined) getNextConfig() {
	// XXX Does it matter that we spin?
	for {
		r.ReadConfig(r.cfgPath, r.cfg)
		// Make sure we've been assigned to a realm
		if r.cfg.RealmId != kernel.NO_REALM {
			break
		}
	}
	// Update the realm lock
	r.realmLock = sync.MakeLock(r.FsLib, named.LOCKS, REALM_LOCK+r.cfg.RealmId, true)
}

// If we need more named replicas, help initialize a realm by starting another
// named replica for it. Return true when all named replicas have been
// initialized.
func (r *Machined) tryAddNamedReplicaL() bool {
	rds, err := r.ReadDir(path.Join(REALMS, r.cfg.RealmId))
	if err != nil {
		log.Fatalf("Error ReadDir in Machined.tryInitRealmL: %v", err)
	}

	initDone := false
	// If this is the last machined replica...
	if len(rds) == nReplicas()-1 {
		initDone = true
	}

	// If we need to add a named replica, do so
	if len(rds) < nReplicas() {
		ip, err := fsclnt.LocalIP()
		if err != nil {
			log.Fatalf("Error LocalIP in Machined.tryInitRealmL: %v", err)
		}
		namedAddrs := genNamedAddrs(1, ip)

		// Get config
		realmCfg := GetRealmConfig(r.FsLib, r.cfg.RealmId)
		realmCfg.NamedAddr = append(realmCfg.NamedAddr, namedAddrs...)

		// Start a named instance.
		var pid string
		if _, pid, err = kernel.BootNamed(r.ProcClnt, r.bin, namedAddrs[0], nReplicas() > 1, len(realmCfg.NamedAddr), realmCfg.NamedAddr, r.cfg.RealmId); err != nil {
			log.Fatalf("Error BootNamed in Machined.tryInitRealmL: %v", err)
		}
		// Update config
		realmCfg.NamedPids = append(realmCfg.NamedPids, pid)
		r.WriteConfig(path.Join(REALM_CONFIG, realmCfg.Rid), realmCfg)

	}
	return initDone
}

// Register this machined as part of a realm.
func (r *Machined) register() {
	// Register this machined as belonging to this realm.
	if err := atomic.MakeFileAtomic(r.FsLib, path.Join(REALMS, r.cfg.RealmId, r.id), 0777, []byte{}); err != nil {
		log.Fatalf("Error MakeFileAtomic in Machined.register: %v", err)
	}
}

func (r *Machined) boot(realmCfg *RealmConfig) {
	r.s = kernel.MakeSystem(r.bin, realmCfg.NamedAddr)
	if err := r.s.Boot(); err != nil {
		log.Fatalf("Error Boot in Machined.boot: %v", err)
	}
}

// Join a realm
func (r *Machined) joinRealm() chan bool {
	r.realmLock.Lock()
	defer r.realmLock.Unlock()

	// Try to initalize this realm if it hasn't been initialized already.
	initDone := r.tryAddNamedReplicaL()
	// Get the realm config
	realmCfg := GetRealmConfig(r.FsLib, r.cfg.RealmId)
	// Register this machined
	r.register()
	// Boot this machined's system services
	r.boot(realmCfg)
	// Signal that the realm has been initialized
	if initDone {
		rStartCond := sync.MakeCond(r.FsLib, path.Join(named.BOOT, r.cfg.RealmId), nil, true)
		rStartCond.Destroy()
	}
	db.DLPrintf("MACHINED", "Machined %v joined Realm %v", r.id, r.cfg.RealmId)
	// Watch for changes to the config
	return r.WatchConfig(r.cfgPath)
}

func (r *Machined) teardown() {
	// Tear down realm resources
	r.s.Shutdown()
}

func (r *Machined) deregister() {
	// De-register this machined as belonging to this realm
	if err := r.Remove(path.Join(REALMS, r.cfg.RealmId, r.id)); err != nil {
		log.Fatalf("Error Remove in Machined.deregister: %v", err)
	}
}

func (r *Machined) tryDestroyRealmL() {
	rds, err := r.ReadDir(path.Join(REALMS, r.cfg.RealmId))
	if err != nil {
		log.Fatalf("Error ReadDir in Machined.tryDestroyRealmL: %v", err)
	}

	// If this is the last machined, destroy the machined's named
	if len(rds) == 0 {
		realmCfg := GetRealmConfig(r.FsLib, r.cfg.RealmId)
		ShutdownNamedReplicas(r.ProcClnt, realmCfg.NamedAddr)

		// Remove the realm config file
		if err := r.Remove(path.Join(REALM_CONFIG, r.cfg.RealmId)); err != nil {
			log.Fatalf("Error Remove in REALM_CONFIG Machined.tryDestroyRealmL: %v", err)
		}

		// Remove the realm directory
		if err := r.RmDir(path.Join(REALMS, r.cfg.RealmId)); err != nil {
			log.Fatalf("Error Remove REALMS in Machined.tryDestroyRealmL: %v", err)
		}

		// Remove the realm's named directory
		if err := r.RmDir(path.Join(REALM_NAMEDS, r.cfg.RealmId)); err != nil {
			log.Fatalf("Error Remove REALMS in Machined.tryDestroyRealmL: %v", err)
		}

		// Signal that the realm has been destroyed
		rExitCond := sync.MakeCond(r.FsLib, path.Join(named.BOOT, r.cfg.RealmId), nil, true)
		rExitCond.Destroy()
	}
}

// Leave a realm
func (r *Machined) leaveRealm() {
	r.realmLock.Lock()
	defer r.realmLock.Unlock()

	db.DLPrintf("MACHINED", "Machined %v leaving Realm %v", r.id, r.cfg.RealmId)

	// Tear down resources
	r.teardown()
	// Deregister this machined
	r.deregister()
	// Try to destroy a realm (if this is the last machined remaining)
	r.tryDestroyRealmL()
}

func (r *Machined) Work() {
	for {
		// Get the next realm assignment.
		r.getNextConfig()

		// Join a realm
		done := r.joinRealm()
		// Wait for the watch to trigger
		<-done

		// Leave a realm
		r.leaveRealm()

		// Mark self as available for allocation.
		r.markFree()
	}
}
