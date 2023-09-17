package sigmaclnt

import (
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
func MkSigmaClntFsLib(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	fsl, err := fslib.NewFsLib(pcfg)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	return MkSigmaLeaseClnt(fsl)
}

func NewSigmaClnt(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	sc, err := MkSigmaClntFsLib(pcfg)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	sc.ProcClnt = procclnt.NewProcClnt(sc.FsLib)
	return sc, nil
}

// Only to be used by non-procs (tests, and linux processes), and creates a
// sigmaclnt for the root realm.
func MkSigmaClntRootInit(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	sc, err := MkSigmaClntFsLib(pcfg)
	if err != nil {
		return nil, err
	}
	sc.ProcClnt = procclnt.NewProcClntInit(pcfg.GetPID(), sc.FsLib, string(pcfg.GetUname()))
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
	sc.ClntExit(proc.NewStatus(proc.StatusOK))
}
