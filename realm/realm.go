package realm

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"path"
	"strconv"

	"ulambda/atomic"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/sync"
)

const (
	DEFAULT_REALM_PRIORITY = "0"
	MIN_PORT               = 1112
	MAX_PORT               = 65535
)

type RealmConfig struct {
	Rid       string // Realm id.
	NamedAddr string // IP address of this realm's named.
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
func (clnt *RealmClnt) CreateRealm(rid string) {
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

	rStartCond.Wait()
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

// Generate an address for a new named
func genNamedAddr(localIP string) string {
	port := strconv.Itoa(MIN_PORT + rand.Intn(MAX_PORT-MIN_PORT))
	return localIP + ":" + port
}

func getRealmConfig(fsl *fslib.FsLib, rid string) *RealmConfig {
	cfg := &RealmConfig{}
	if err := fsl.ReadFileJson(path.Join(REALM_CONFIG, rid), cfg); err != nil {
		log.Fatalf("Error ReadFileJson in getRealmConfig: %v, %v", path.Join(REALM_CONFIG, rid), err)
	}
	return cfg
}

func setRealmConfig(fsl *fslib.FsLib, cfg *RealmConfig) {
	if err := atomic.MakeFileJsonAtomic(fsl, path.Join(REALM_CONFIG, cfg.Rid), 0777, cfg); err != nil {
		log.Fatalf("Error ReadFileJson in setRealmConfig: %v", err)
	}
}
