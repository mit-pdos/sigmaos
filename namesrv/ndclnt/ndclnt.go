package ndclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	//"sigmaos/test"
)

const MCPU proc.Tmcpu = 1000

func NewNamedProc(realm sp.Trealm, dialproxy bool, canFail bool) *proc.Proc {
	p := proc.NewProc(sp.NAMEDREL, []string{realm.String()})
	p.SetMcpu(MCPU)
	p.SetRealmSwitch(realm)
	p.GetProcEnv().UseDialProxy = dialproxy
	if !canFail {
		p.AppendEnv("SIGMAFAIL", "")
	} else {
		p.AppendEnv(proc.SIGMAFAIL, proc.GetSigmaFail())
	}
	return p
}

func StartNamed(sc *sigmaclnt.SigmaClnt, nd *proc.Proc, pn string) error {
	if err := sc.Spawn(nd); err != nil {
		return err
	}
	if err := sc.WaitStart(nd.GetPid()); err != nil {
		return err
	}
	if err := WaitNamed(sc.FsLib, pn); err != nil {
		return err
	}
	db.DPrintf(db.TEST, "New named ready to serve")
	return nil
}

func StartNamedGrp(sc *sigmaclnt.SigmaClnt, cfg *procgroupmgr.ProcGroupMgrConfig) *procgroupmgr.ProcGroupMgr {
	db.DPrintf(db.NAMED_LDR, "StartNamedGrp %v spawn named", cfg)
	return cfg.StartGrpMgr(sc)
}

// wait until the realm's named has registered its endpoint and is ready to
// serve
func WaitNamed(fsl *fslib.FsLib, pn string) error {
	if b, err := fsl.GetFileWatch(pn); err != nil {
		return err
	} else {
		ep, err := sp.NewEndpointFromBytes(b)
		if err != nil {
			db.DPrintf(db.NAMED_LDR, "named ep %v err %v", string(b), err)
		}
		db.DPrintf(db.NAMED_LDR, "named ep %v", ep)
	}
	return nil
}

func StopNamed(sc *sigmaclnt.SigmaClnt, nd *proc.Proc) error {
	// Evict the new named
	if err := sc.Evict(nd.GetPid()); err != nil {
		return err
	}
	status, err := sc.WaitExit(nd.GetPid())
	if err != nil {
		return err
	}
	if !status.IsStatusEvicted() {
		return fmt.Errorf("Wrong exit status %v", status)
	}
	return err
}

// Remove the named ep file for the new realm, in case the realm name
// is re-used and it hasn't been clean removed yet (e.g., by
// realmd). Once it is removed, we can watch for it to detect if the
// named of the new incarnation of the realm exists. (The EP file
// isn't leased and thus not automatically deleted.)
func RemoveNamedEP(fsl *fslib.FsLib, pn string) error {
	err := fsl.Remove(pn)
	return err
}
