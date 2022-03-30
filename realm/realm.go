package realm

import (
	"fmt"
	"path"
	"time"

	"ulambda/config"
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/procclnt"
	"ulambda/semclnt"
)

const (
	DEFAULT_REALM_PRIORITY = "0"
	MIN_PORT               = 1112
	MAX_PORT               = 60000
)

type RealmConfig struct {
	Rid        string    // Realm id.
	NMachineds int       // Number of machineds currently assigned to this realm.
	LastResize time.Time // Timestamp from the last time this realm was resized
	Shutdown   bool      // True if this realm is in the process of being destroyed.
	NamedAddr  []string  // IP address of this realm's nameds.
	NamedPids  []string  // PIDs of this realm's nameds.
}

type RealmClnt struct {
	*procclnt.ProcClnt
	*fslib.FsLib
}

func MakeRealmClnt() *RealmClnt {
	clnt := &RealmClnt{}
	clnt.FsLib = fslib.MakeFsLib(fmt.Sprintf("realm-clnt"))
	clnt.ProcClnt = procclnt.MakeProcClntInit(clnt.FsLib, "realm-clnt", fslib.Named())
	return clnt
}

// Submit a realm creation request to the realm manager, and wait for the
// request to be handled.
func (clnt *RealmClnt) CreateRealm(rid string) *RealmConfig {
	// Create semaphore to wait on realm creation/initialization.
	rStartSem := semclnt.MakeSemClnt(clnt.FsLib, path.Join(np.BOOT, rid))
	rStartSem.Init(0)

	if _, err := clnt.SetFile(REALM_CREATE, []byte(rid), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile in RealmClnt.CreateRealm: %v", err)
	}

	// Wait for the realm to be initialized
	rStartSem.Down()

	return GetRealmConfig(clnt.FsLib, rid)
}

func (clnt *RealmClnt) DestroyRealm(rid string) {
	// Create cond var to wait on realm creation/initialization.
	rExitSem := semclnt.MakeSemClnt(clnt.FsLib, path.Join(np.BOOT, rid))
	rExitSem.Init(0)

	if _, err := clnt.SetFile(REALM_DESTROY, []byte(rid), np.OWRITE, 0); err != nil {
		db.DFatalf("Error WriteFile in RealmClnt.DestroyRealm: %v", err)
	}

	rExitSem.Down()
}

// Get a realm's configuration
func GetRealmConfig(fsl *fslib.FsLib, rid string) *RealmConfig {
	clnt := config.MakeConfigClnt(fsl)
	cfg := &RealmConfig{}
	clnt.ReadConfig(path.Join(REALM_CONFIG, rid), cfg)
	return cfg
}
