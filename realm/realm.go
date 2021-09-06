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
	"ulambda/kernel"
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

type Realm struct {
	cfg *RealmConfig
	*fslib.FsLib
}

// Submit a realm allocation request to the realm manager, and wait for the
// request to be handled.
func MakeRealm(rid string) *Realm {
	r := &Realm{}
	r.cfg = &RealmConfig{}
	r.cfg.Rid = rid
	r.FsLib = fslib.MakeFsLib(fmt.Sprintf("realm-%v", rid))

	rStartCond := sync.MakeCond(r.FsLib, path.Join(kernel.BOOT, rid), nil)
	rStartCond.Init()

	b, err := json.Marshal(r)
	if err != nil {
		log.Fatalf("Error Marshal in MakeRealm: %v", err)
	}

	alloc := sync.MakeFilePriorityBag(r.FsLib, REALM_ALLOC)
	if err := alloc.Put(DEFAULT_REALM_PRIORITY, rid, b); err != nil {
		log.Fatalf("Error Put in MakeRealm: %v", err)
	}

	rStartCond.Wait()

	return r
}

func (r *Realm) Destroy() {
	log.Fatalf("Error: DestroyRealm unimplemented")
	// TODO: remove a named
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
