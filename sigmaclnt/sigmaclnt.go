package sigmaclnt

import (
	"strings"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leaseclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

func init() {
	if db.WillBePrinted(db.SPAWN_LAT) {
		name := proc.GetSigmaDebugPid()
		// Don't print for test programs, which won't have a debug PID set.
		if name == "" || strings.Contains(name, "test-") {
			return
		}
		pe := proc.GetProcEnv()
		db.DPrintf(db.SPAWN_LAT, "[%v] SigmaClnt pkg init. E2e spawn latency: %v", pe.GetPID(), time.Since(pe.GetSpawnTime()))
	}
}

type SigmaClnt struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*leaseclnt.LeaseClnt
}

func NewSigmaLeaseClnt(fsl *fslib.FsLib) (*SigmaClnt, error) {
	lmc, err := leaseclnt.NewLeaseClnt(fsl)
	if err != nil {
		return nil, err
	}
	return &SigmaClnt{fsl, nil, lmc}, nil
}

// Create only an FsLib, as a proc.
func NewSigmaClntFsLib(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	fsl, err := fslib.NewFsLib(pcfg)
	if err != nil {
		db.DFatalf("NewSigmaClnt: %v", err)
	}
	return NewSigmaLeaseClnt(fsl)
}

func NewSigmaClnt(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	start := time.Now()
	sc, err := NewSigmaClntFsLib(pcfg)
	if err != nil {
		db.DFatalf("NewSigmaClnt: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Make FsLib: %v", pcfg.GetPID(), time.Since(start))
	start = time.Now()
	sc.ProcClnt = procclnt.NewProcClnt(sc.FsLib)
	db.DPrintf(db.SPAWN_LAT, "[%v] Make ProcClnt: %v", pcfg.GetPID(), time.Since(start))
	return sc, nil
}

// Only to be used by non-procs (tests, and linux processes), and creates a
// sigmaclnt for the root realm.
func NewSigmaClntRootInit(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	sc, err := NewSigmaClntFsLib(pcfg)
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
