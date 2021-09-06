package realm

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/sync"
)

const (
	DEFAULT_REALM_PRIORITY = "0"
)

type Realm struct {
	Rid     string // Realm id.
	NamedIP string // IP address of this realm's named.
	*fslib.FsLib
}

// Submit a realm allocation request to the realm manager, and wait for the
// request to be handled.
func MakeRealm(rid string) *Realm {
	r := &Realm{}
	r.Rid = rid
	r.FsLib = fslib.MakeFsLib(fmt.Sprintf("realm-%v", r.Rid))

	log.Fatalf("Error: CreateRealm unimplemented")

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
