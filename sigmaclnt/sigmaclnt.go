package sigmaclnt

import (
	"strings"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leaseclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/sigmaclntclnt"
	// sos "sigmaos/sigmaos"
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
	proc.ProcAPI
	*leaseclnt.LeaseClnt
}

type SigmaClntKernel struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*leaseclnt.LeaseClnt
}

// Convert to SigmaClntKernel from SigmaClnt
func NewSigmaClntKernel(sc *SigmaClnt) *SigmaClntKernel {
	sck := &SigmaClntKernel{sc.FsLib, sc.ProcAPI.(*procclnt.ProcClnt), sc.LeaseClnt}
	return sck
}

// Convert to SigmaClnt from SigmaClntKernel
func NewSigmaClntProcAPI(sck *SigmaClntKernel) *SigmaClnt {
	sc := &SigmaClnt{sck.FsLib, sck.ProcClnt, sck.LeaseClnt}
	return sc
}

// Create only an FsLib (using fdclient), as a proc.
func NewSigmaClntFsLib(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	fsl, err := fslib.NewFsLib(pcfg)
	if err != nil {
		db.DFatalf("NewSigmaClnt: %v", err)
	}
	lmc, err := leaseclnt.NewLeaseClnt(fsl)
	if err != nil {
		return nil, err
	}
	return &SigmaClnt{fsl, nil, lmc}, nil
}

// Create a SigmaClnt using usigmaclntd or fdclnt
func newSigmaClntClnt(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	var fsl *fslib.FsLib
	var err error
	if pcfg.UseSigmaclntd {
		scc, err := sigmaclntclnt.NewSigmaClntClnt()
		if err != nil {
			db.DPrintf(db.ALWAYS, "newSigmaClntClnt err %v", err)
			return nil, err
		}
		fsl, err = fslib.NewFsLibAPI(pcfg, scc)
	} else {
		fsl, err = fslib.NewFsLib(pcfg)
	}
	if err != nil {
		db.DPrintf(db.ALWAYS, "NewFsLibAPI err %v", err)
	}
	lmc, err := leaseclnt.NewLeaseClnt(fsl)
	if err != nil {
		return nil, err
	}
	return &SigmaClnt{fsl, nil, lmc}, nil
}

func NewSigmaClnt(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	start := time.Now()
	sc, err := newSigmaClntClnt(pcfg)
	if err != nil {
		db.DFatalf("NewSigmaClnt: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Make FsLib: %v", pcfg.GetPID(), time.Since(start))
	start = time.Now()
	sc.ProcAPI = procclnt.NewProcClnt(sc.FsLib)
	db.DPrintf(db.SPAWN_LAT, "[%v] Make ProcClnt: %v", pcfg.GetPID(), time.Since(start))
	return sc, nil
}

// Only to be used by non-procs (tests, and linux processes), and creates a
// sigmaclnt for the root realm.
func NewSigmaClntRootInit(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	sc, err := newSigmaClntClnt(pcfg)
	if err != nil {
		return nil, err
	}
	sc.ProcAPI = procclnt.NewProcClntInit(pcfg.GetPID(), sc.FsLib, string(pcfg.GetUname()))
	return sc, nil
}

func (sc *SigmaClnt) ClntExit(status *proc.Status) error {
	sc.ProcAPI.Exited(status)
	db.DPrintf(db.SIGMACLNT, "Exited done")
	if sc.LeaseClnt != nil {
		sc.LeaseClnt.EndLeases()
	}
	db.DPrintf(db.SIGMACLNT, "EndLeases done")
	defer db.DPrintf(db.SIGMACLNT, "ClntExit done")
	return sc.FsLib.DetachAll()
}

func (sc *SigmaClnt) ClntExitOK() {
	sc.ClntExit(proc.NewStatus(proc.StatusOK))
}
