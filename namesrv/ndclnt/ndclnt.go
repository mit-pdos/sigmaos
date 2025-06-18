package ndclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const MCPU proc.Tmcpu = 1000

type NdClnt struct {
	sc *sigmaclnt.SigmaClnt
	pn string
}

func NewNdClnt(sc *sigmaclnt.SigmaClnt, pn string) *NdClnt {
	return &NdClnt{
		sc: sc,
		pn: pn,
	}
}

func (ndc *NdClnt) Name() string {
	return ndc.pn
}

// Remove the named ep file for the new realm, in case the realm name
// is re-used and it hasn't been clean removed yet (e.g., by
// realmd). Once it is removed, we can watch for it to detect if the
// named of the new incarnation of the realm exists. (The EP file
// isn't leased and thus not automatically deleted.)
func (ndc *NdClnt) RemoveNamedEP() error {
	err := ndc.sc.Remove(ndc.pn)
	return err
}

// Wait until the realm's named has registered its endpoint and is
// ready to serve
func (ndc *NdClnt) WaitNamed() error {
	if b, err := ndc.sc.GetFileWatch(ndc.pn); err != nil {
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

func (ndc *NdClnt) ClearAndStartNamed(nd *proc.Proc) error {
	ndc.RemoveNamedEP()
	if err := ndc.sc.Spawn(nd); err != nil {
		return err
	}
	if err := ndc.sc.WaitStart(nd.GetPid()); err != nil {
		return err
	}
	if err := ndc.WaitNamed(); err != nil {
		return err
	}
	db.DPrintf(db.TEST, "New named ready to serve")
	return nil
}

func (ndc *NdClnt) StartNamed(nd *proc.Proc) error {
	if err := ndc.sc.Spawn(nd); err != nil {
		return err
	}
	if err := ndc.sc.WaitStart(nd.GetPid()); err != nil {
		return err
	}
	db.DPrintf(db.TEST, "New named spawned")
	return nil
}

func (ndc *NdClnt) StopNamed(nd *proc.Proc) error {
	// Evict the named
	if err := ndc.sc.Evict(nd.GetPid()); err != nil {
		return err
	}
	status, err := ndc.sc.WaitExit(nd.GetPid())
	if err != nil {
		return err
	}
	if !status.IsStatusEvicted() {
		return fmt.Errorf("Wrong exit status %v", status)
	}
	return err
}
