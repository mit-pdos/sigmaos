package realm

import (
	"fmt"
	"path"

	"ulambda/config"
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/resource"
	"ulambda/semclnt"
)

const (
	DEFAULT_REALM_PRIORITY = "0"
	MIN_PORT               = 1112
	MAX_PORT               = 60000
)

type RealmClnt struct {
	*procclnt.ProcClnt
	*fslib.FsLib
}

func MakeRealmClnt() *RealmClnt {
	clnt := &RealmClnt{}
	clnt.FsLib = fslib.MakeFsLib(fmt.Sprintf("realm-clnt"))
	clnt.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), clnt.FsLib, "realm-clnt", fslib.Named())
	return clnt
}

// Submit a realm creation request to the realm manager, and wait for the
// request to be handled.
func (clnt *RealmClnt) CreateRealm(rid string) *RealmConfig {
	// Create semaphore to wait on realm creation/initialization.
	rStartSem := semclnt.MakeSemClnt(clnt.FsLib, path.Join(np.BOOT, rid))
	rStartSem.Init(0)

	msg := resource.MakeResourceMsg(resource.Trequest, resource.Trealm, rid, 1)

	if _, err := clnt.SetFile(SIGMACTL, msg.Marshal(), np.OWRITE, 0); err != nil {
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

	msg := resource.MakeResourceMsg(resource.Tgrant, resource.Trealm, rid, 1)

	if _, err := clnt.SetFile(SIGMACTL, msg.Marshal(), np.OWRITE, 0); err != nil {
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
