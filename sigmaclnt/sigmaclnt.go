// Package sigmaclnt implements the client interface of SigmaOS: the
// proc API, the file API, and lease client.
package sigmaclnt

import (
	"strings"
	"time"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/fdclnt"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	leaseclnt "sigmaos/lease/clnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sos "sigmaos/sigmaos"
	spproxyclnt "sigmaos/spproxy/clnt"
)

func init() {
	if db.IsLabelSet(db.SPAWN_LAT) {
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
	sc *SigmaClnt
}

// Create FsLib using either sigmacntclnt or fdclnt
func newFsLibFidClnt(pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*fslib.FsLib, error) {
	var err error
	var s sos.FileAPI
	if pe.UseSPProxy {
		db.DPrintf(db.ALWAYS, "newSPProxyClnt init from proxy")
		s, err = spproxyclnt.NewSPProxyClnt(pe, fidc.GetDialProxyClnt())
		if err != nil {
			db.DPrintf(db.ALWAYS, "newSPProxyClnt err %v", err)
			return nil, err
		}
	} else {
		db.DPrintf(db.ALWAYS, "NewFdClint init")
		s = fdclnt.NewFdClient(pe, fidc)
	}
	return fslib.NewFsLibAPI(pe, fidc.GetDialProxyClnt(), s)
}

func NewFsLib(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt) (*fslib.FsLib, error) {
	return newFsLibFidClnt(pe, fidclnt.NewFidClnt(pe, npc))
}

// Convert to SigmaClntKernel from SigmaClnt
func NewSigmaClntKernel(sc *SigmaClnt) *SigmaClntKernel {
	sck := &SigmaClntKernel{sc.FsLib, sc.ProcAPI.(*procclnt.ProcClnt), sc.LeaseClnt, sc}
	return sck
}

func (sck *SigmaClntKernel) SigmaClnt() *SigmaClnt {
	return sck.sc
}

// Convert to SigmaClnt from SigmaClntKernel
func NewSigmaClntProcAPI(sck *SigmaClntKernel) *SigmaClnt {
	sc := &SigmaClnt{
		FsLib:     sck.FsLib,
		ProcAPI:   sck.ProcClnt,
		LeaseClnt: sck.LeaseClnt,
	}
	return sc
}

// Create a SigmaClnt (using spproxyclnt or fdclient), as a proc, without ProcAPI.
func NewSigmaClntFsLibFidClnt(pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClnt, error) {
	fidc.NewClnt()
	fsl, err := newFsLibFidClnt(pe, fidc)
	if err != nil {
		db.DPrintf(db.ERROR, "NewSigmaClnt: %v", err)
		return nil, err
	}
	lmc, err := leaseclnt.NewLeaseClnt(fsl)
	//0xc0031a6aa0
	db.DPrintf(db.ALWAYS, "New LeaseClnt %p", lmc)
	if err != nil {
		return nil, err
	}
	return &SigmaClnt{
		FsLib:     fsl,
		ProcAPI:   nil,
		LeaseClnt: lmc,
	}, nil
}

func NewSigmaClntFsLib(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt) (*SigmaClnt, error) {
	return NewSigmaClntFsLibFidClnt(pe, fidclnt.NewFidClnt(pe, npc))
}

func NewSigmaClnt(pe *proc.ProcEnv) (*SigmaClnt, error) {
	start := time.Now()
	sc, err := NewSigmaClntFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	if err != nil {
		db.DPrintf(db.ERROR, "NewSigmaClnt: %v", err)
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Make FsLib: %v", pe.GetPID(), time.Since(start))
	if err := sc.NewProcClnt(); err != nil {
		return nil, err
	}
	return sc, nil
}

// Only to be used by non-procs (tests, and linux processes), and creates a
// sigmaclnt for the root realm.
func NewSigmaClntRootInit(pe *proc.ProcEnv) (*SigmaClnt, error) {
	sc, err := NewSigmaClntFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	if err != nil {
		return nil, err
	}
	papi, err := procclnt.NewProcClntInit(pe.GetPID(), sc.FsLib, pe.GetPrincipal().GetID().String())
	if err != nil {
		return nil, err
	}
	sc.ProcAPI = papi
	return sc, nil
}

func (sc *SigmaClnt) NewProcClnt() error {
	start := time.Now()
	db.DPrintf(db.SPAWN_LAT, "[%v] preparing ProcClnt: %v", sc.ProcEnv().GetPID(), time.Since(start))
	papi, err := procclnt.NewProcClnt(sc.FsLib)
	if err != nil {
		db.DPrintf(db.ERROR, "NewProcClnt: %v", err)
		return err
	}
	sc.ProcAPI = papi
	db.DPrintf(db.SPAWN_LAT, "[%v] Make ProcClnt: %v", sc.ProcEnv().GetPID(), time.Since(start))
	return nil
}

func (sc *SigmaClnt) ClntExit(status *proc.Status) error {
	db.DPrintf(db.SIGMACLNT, "Exiting %v", status)
	sc.ProcAPI.Exited(status)
	if sc.LeaseClnt != nil {
		sc.LeaseClnt.EndLeases()
	}
	db.DPrintf(db.SIGMACLNT, "EndLeases done")
	defer db.DPrintf(db.SIGMACLNT, "ClntExit done")
	return sc.FsLib.Close()
}

func (sc *SigmaClnt) CloseNew() error {
	status := proc.NewStatus(proc.StatusOK)
	db.DPrintf(db.SIGMACLNT, "Closing %v", status)
	sc.ProcAPI.Exited(status)
	//if sc.LeaseClnt != nil {
	//		sc.LeaseClnt.EndLeases()
	//	}
	//	db.DPrintf(db.SIGMACLNT, "EndLeases done")
	defer db.DPrintf(db.SIGMACLNT, "ClntExit done")
	return sc.FsLib.Close()
}

// must  have lease clnt in newsc
func (sc *SigmaClnt) Close2(newsc *SigmaClnt) error {
	status := proc.NewStatus(proc.StatusOK)
	db.DPrintf(db.SIGMACLNT, "Closing %v", status)
	sc.ProcAPI.Exited(status)
	if sc.LeaseClnt != nil {
		sc.LeaseClnt.EndLeases()
		//sc.LeaseClnt.EndLeasesNewClient(newsc.LeaseClnt)
	}
	//	db.DPrintf(db.SIGMACLNT, "EndLeases done")
	defer db.DPrintf(db.SIGMACLNT, "ClntExit done")
	return sc.FsLib.Close()
}
func (sc *SigmaClnt) ClntExitOK() {
	sc.ClntExit(proc.NewStatus(proc.StatusOK))
}

func (sc *SigmaClnt) StopWatchingSrvs() {
	sc.ProcAPI.(*procclnt.ProcClnt).StopWatchingSrvs()
}
