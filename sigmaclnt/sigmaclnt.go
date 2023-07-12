package sigmaclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leasemgrclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

type SigmaClnt struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*leasemgrclnt.LeaseMgrClnt
}

func MkSigmaLeaseClnt(fsl *fslib.FsLib) (*SigmaClnt, error) {
	lmc, err := leasemgrclnt.NewLeaseMgrClnt(fsl)
	if err != nil {
		return nil, err
	}
	return &SigmaClnt{fsl, nil, lmc}, nil
}

// Create only an FsLib, as a proc.
func MkSigmaClntFsLib(uname sp.Tuname) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLib(uname)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	return MkSigmaLeaseClnt(fsl)
}

// Only to be called by procs (uses SIGMAREALM env variable, and expects realm
// namespace to be set up for this proc, e.g. procdir).
func MkSigmaClnt(uname sp.Tuname) (*SigmaClnt, error) {
	sc, err := MkSigmaClntFsLib(uname)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	sc.ProcClnt = procclnt.MakeProcClnt(sc.FsLib)
	return sc, nil
}

// Create only an FsLib, relative to a realm, but with the client being in the root realm
func MkSigmaClntRealmFsLib(rootrealm *fslib.FsLib, uname sp.Tuname, rid sp.Trealm) (*SigmaClnt, error) {
	db.DPrintf(db.SIGMACLNT, "Realm %v NamedAddr %v\n", rid, nil)
	realm, err := fslib.MakeFsLibAddrNet(uname, rid, rootrealm.GetLocalIP(), nil, sp.ROOTREALM.String())
	if err != nil {
		db.DPrintf(db.SIGMACLNT, "Error mkFsLibAddr [%v]: %v", nil, err)
		return nil, err
	}
	return MkSigmaLeaseClnt(realm)
}

// Create a full sigmaclnt relative to a realm (fslib and procclnt)
func MkSigmaClntRealm(rootfsl *fslib.FsLib, uname sp.Tuname, rid sp.Trealm) (*SigmaClnt, error) {
	db.DPrintf(db.SIGMACLNT, "MkSigmaClntRealmProc %v\n", rid)
	sc, err := MkSigmaClntRealmFsLib(rootfsl, uname, rid)
	if err != nil {
		return nil, err
	}
	sc.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), sc.FsLib, string(uname))
	return sc, nil
}

// Only to be used by non-procs (tests, and linux processes), and creates a
// sigmaclnt for the root realm.
func MkSigmaClntRootInit(uname sp.Tuname, ip string, namedAddr sp.Taddrs) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLibAddrNet(uname, sp.ROOTREALM, ip, namedAddr, sp.ROOTREALM.String())
	if err != nil {
		return nil, err
	}
	sc, err := MkSigmaLeaseClnt(fsl)
	if err != nil {
		return nil, err
	}
	sc.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), fsl, string(uname))
	return sc, nil
}

func (sc *SigmaClnt) Exit(status *proc.Status) error {
	sc.ProcClnt.Exited(status)
	if sc.LeaseMgrClnt != nil {
		sc.LeaseMgrClnt.EndLeases()
	}
	return sc.FsLib.DetachAll()
}

func (sc *SigmaClnt) ExitOK() {
	sc.Exit(proc.MakeStatus(proc.StatusOK))
}
