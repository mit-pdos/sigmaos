package realm

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/config"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/sync"
)

const (
	DEFAULT_REALM_PRIORITY = "0"
	MIN_PORT               = 1112
	MAX_PORT               = 60000
)

type RealmConfig struct {
	Rid       string   // Realm id.
	NRealmds  int      // Number of realmds currently assigned to this realm.
	Shutdown  bool     // True if this realm is in the process of being destroyed.
	NamedAddr []string // IP address of this realm's named.
}

type RealmClnt struct {
	create  *sync.FilePriorityBag
	destroy *sync.FilePriorityBag
	*fslib.FsLib
}

func MakeRealmClnt() *RealmClnt {
	clnt := &RealmClnt{}
	clnt.FsLib = fslib.MakeFsLib(fmt.Sprintf("realm-clnt"))
	clnt.create = sync.MakeFilePriorityBag(clnt.FsLib, REALM_CREATE)
	clnt.destroy = sync.MakeFilePriorityBag(clnt.FsLib, REALM_DESTROY)
	return clnt
}

// Submit a realm creation request to the realm manager, and wait for the
// request to be handled.
func (clnt *RealmClnt) CreateRealm(rid string) *RealmConfig {
	cfg := &RealmConfig{}
	cfg.Rid = rid

	// Create cond var to wait on realm creation/initialization.
	rStartCond := sync.MakeCond(clnt.FsLib, path.Join(named.BOOT, rid), nil)
	rStartCond.Init()

	b, err := json.Marshal(cfg)
	if err != nil {
		log.Fatalf("Error Marshal in RealmClnt.CreateRealm: %v", err)
	}

	if err := clnt.create.Put(DEFAULT_REALM_PRIORITY, rid, b); err != nil {
		log.Fatalf("Error Put in RealmClnt.CreateRealm: %v", err)
	}

	// Wait for the realm to be initialized
	rStartCond.Wait()

	return GetRealmConfig(clnt.FsLib, rid)
}

func (clnt *RealmClnt) DestroyRealm(rid string) {
	// Create cond var to wait on realm creation/initialization.
	rExitCond := sync.MakeCond(clnt.FsLib, path.Join(named.BOOT, rid), nil)
	rExitCond.Init()

	if err := clnt.destroy.Put(DEFAULT_REALM_PRIORITY, rid, []byte{}); err != nil {
		log.Fatalf("Error Put in RealmClnt.DestroyRealm: %v", err)
	}

	rExitCond.Wait()
}

// Get a realm's configuration
func GetRealmConfig(fsl *fslib.FsLib, rid string) *RealmConfig {
	clnt := config.MakeConfigClnt(fsl)
	cfg := &RealmConfig{}
	clnt.ReadConfig(path.Join(REALM_CONFIG, rid), cfg)
	return cfg
}
