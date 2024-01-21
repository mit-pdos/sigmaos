package sigmaclnt

import (
	"strings"
	"time"

	db "sigmaos/debug"
	"sigmaos/fdclnt"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	"sigmaos/leaseclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/sigmaclntclnt"
	sos "sigmaos/sigmaos"
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

// Create FsLib using either sigmalcntd or fdclnt
func newFsLibFidClnt(pcfg *proc.ProcEnv, fidc *fidclnt.FidClnt) (*fslib.FsLib, error) {
	var err error
	var s sos.SigmaOS
	if pcfg.UseSigmaclntd {
		s, err = sigmaclntclnt.NewSigmaClntClnt()
		if err != nil {
			db.DPrintf(db.ALWAYS, "newSigmaClntClnt err %v", err)
			return nil, err
		}
	} else {
		s = fdclnt.NewFdClient(pcfg, fidc)
	}
	return fslib.NewFsLibAPI(pcfg, s)
}

func NewFsLib(pcfg *proc.ProcEnv) (*fslib.FsLib, error) {
	return newFsLibFidClnt(pcfg, nil)
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

// Create a SigmaClnt (using sigmaclntd or fdclient), as a proc, without ProcAPI.
func NewSigmaClntFsLibFidClnt(pcfg *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClnt, error) {
	fsl, err := newFsLibFidClnt(pcfg, fidc)
	if err != nil {
		db.DFatalf("NewSigmaClnt: %v", err)
	}
	lmc, err := leaseclnt.NewLeaseClnt(fsl)
	if err != nil {
		return nil, err
	}
	return &SigmaClnt{fsl, nil, lmc}, nil
}

func NewSigmaClntFsLib(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	return NewSigmaClntFsLibFidClnt(pcfg, nil)
}

func NewSigmaClnt(pcfg *proc.ProcEnv) (*SigmaClnt, error) {
	start := time.Now()
	sc, err := NewSigmaClntFsLib(pcfg)
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
	sc, err := NewSigmaClntFsLib(pcfg)
	if err != nil {
		return nil, err
	}
	papi, err := procclnt.NewProcClntInit(pcfg.GetPID(), sc.FsLib, string(pcfg.GetPrincipal()))
	if err != nil {
		return nil, err
	}
	sc.ProcAPI = papi
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
	return sc.FsLib.Close()
}

func (sc *SigmaClnt) ClntExitOK() {
	sc.ClntExit(proc.NewStatus(proc.StatusOK))
}
