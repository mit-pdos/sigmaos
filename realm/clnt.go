package realm

import (
	"fmt"
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
	"sigmaos/realm/proto"
	"sigmaos/semclnt"
	np "sigmaos/sigmap"
)

const (
	DEFAULT_REALM_PRIORITY = "0"
	MIN_PORT               = 1112
	MAX_PORT               = 60000
)

type RealmClnt struct {
	sclnt *protdevclnt.ProtDevClnt
	*procclnt.ProcClnt
	*fslib.FsLib
}

func MakeRealmClnt() *RealmClnt {
	clnt := &RealmClnt{}
	clnt.FsLib = fslib.MakeFsLib(fmt.Sprintf("realm-clnt"))
	clnt.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), clnt.FsLib, "realm-clnt", fslib.Named())
	return clnt
}

func MakeRealmClntFsl(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) *RealmClnt {
	return &RealmClnt{nil, pclnt, fsl}
}

// Submit a realm creation request to the realm manager, and wait for the
// request to be handled.
func (clnt *RealmClnt) CreateRealm(rid string) *RealmConfig {
	if clnt.sclnt == nil {
		var err error
		clnt.sclnt, err = protdevclnt.MkProtDevClnt(clnt.FsLib, np.SIGMAMGR)
		if err != nil {
			db.DFatalf("Error MkProtDevClnt: %v", err)
		}
	}
	// Create semaphore to wait on realm creation/initialization.
	rStartSem := semclnt.MakeSemClnt(clnt.FsLib, path.Join(np.BOOT, rid))
	rStartSem.Init(0)

	res := &proto.SigmaMgrResponse{}
	req := &proto.SigmaMgrRequest{
		RealmId: rid,
	}
	err := clnt.sclnt.RPC("SigmaMgr.CreateRealm", req, res)
	if err != nil || !res.OK {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}

	// Wait for the realm to be initialized
	rStartSem.Down()

	return GetRealmConfig(clnt.FsLib, rid)
}

// Artificially grow a realm. Mainly used for testing purposes.
func (clnt *RealmClnt) GrowRealm(rid string) {
	if clnt.sclnt == nil {
		var err error
		clnt.sclnt, err = protdevclnt.MkProtDevClnt(clnt.FsLib, np.SIGMAMGR)
		if err != nil {
			db.DFatalf("Error MkProtDevClnt: %v", err)
		}
	}
	db.DPrintf("REALMCLNT", "Artificially grow realm %v", rid)
	res := &proto.SigmaMgrResponse{}
	req := &proto.SigmaMgrRequest{
		RealmId: rid,
		Qlen:    1,
	}
	err := clnt.sclnt.RPC("SigmaMgr.RequestCores", req, res)
	if err != nil || !res.OK {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}
}

func (clnt *RealmClnt) DestroyRealm(rid string) {
	// Create cond var to wait on realm creation/initialization.
	rExitSem := semclnt.MakeSemClnt(clnt.FsLib, path.Join(np.BOOT, rid))
	rExitSem.Init(0)

	res := &proto.SigmaMgrResponse{}
	req := &proto.SigmaMgrRequest{
		RealmId: rid,
	}
	err := clnt.sclnt.RPC("SigmaMgr.DestroyRealm", req, res)
	if err != nil || !res.OK {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}

	rExitSem.Down()
}
