package sigmaclnt

import (
	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leaseclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

type SigmaClnt struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*leaseclnt.LeaseClnt
}

func MkSigmaLeaseClnt(fsl *fslib.FsLib) (*SigmaClnt, error) {
	lmc, err := leaseclnt.NewLeaseClnt(fsl)
	if err != nil {
		return nil, err
	}
	return &SigmaClnt{fsl, nil, lmc}, nil
}

// Create only an FsLib, as a proc.
func MkSigmaClntFsLib(scfg *config.SigmaConfig) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLib(scfg)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	return MkSigmaLeaseClnt(fsl)
}

func NewSigmaClnt(scfg *config.SigmaConfig) (*SigmaClnt, error) {
	sc, err := MkSigmaClntFsLib(scfg)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	sc.ProcClnt = procclnt.MakeProcClnt(sc.FsLib)
	return sc, nil
}

// // Create only an FsLib, relative to a realm, but with the client being in the root realm
//
//	func MkSigmaClntRealmFsLib(rootrealm *fslib.FsLib, uname sp.Tuname, rid sp.Trealm) (*SigmaClnt, error) {
//		db.DPrintf(db.SIGMACLNT, "Realm %v\n", rid)
//		scfg := config.NewTestSigmaConfig(rid, rootrealm.SigmaConfig().EtcdIP, rootrealm.SigmaConfig().LocalIP, rootrealm.SigmaConfig().BuildTag)
//		db.DPrintf(db.SIGMACLNT, "Realm %v config %v\n", rid, scfg)
//
//		fsl, err := fslib.MakeFsLib(scfg)
//		if err != nil {
//			db.DPrintf(db.SIGMACLNT, "Error mkFsLibAddr [%v]: %v", nil, err)
//			return nil, err
//		}
//		return MkSigmaLeaseClnt(fsl)
//	}

//// Create a full sigmaclnt relative to a realm (fslib and procclnt)
//func MkSigmaClntRealm(rootfsl *fslib.FsLib, uname sp.Tuname, rid sp.Trealm) (*SigmaClnt, error) {
//	db.DPrintf(db.SIGMACLNT, "MkSigmaClntRealmProc %v\n", rid)
//	sc, err := MkSigmaClntRealmFsLib(rootfsl, uname, rid)
//	if err != nil {
//		return nil, err
//	}
//	sc.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), sc.FsLib, string(uname))
//	return sc, nil
//}

// Only to be used by non-procs (tests, and linux processes), and creates a
// sigmaclnt for the root realm.
func MkSigmaClntRootInit(scfg *config.SigmaConfig) (*SigmaClnt, error) {
	sc, err := MkSigmaClntFsLib(scfg)
	if err != nil {
		return nil, err
	}

	sc.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), sc.FsLib, string(scfg.Uname))
	return sc, nil
}

func (sc *SigmaClnt) ClntExit(status *proc.Status) error {
	sc.ProcClnt.Exited(status)
	if sc.LeaseClnt != nil {
		sc.LeaseClnt.EndLeases()
	}
	return sc.FsLib.DetachAll()
}

func (sc *SigmaClnt) ClntExitOK() {
	sc.ClntExit(proc.MakeStatus(proc.StatusOK))
}
