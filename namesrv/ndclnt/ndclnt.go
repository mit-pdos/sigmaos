package ndclnt

import (
	"fmt"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const MCPU proc.Tmcpu = 1000

type NdClnt struct {
	scRoot *sigmaclnt.SigmaClnt
	pn     string
}

func NewNdClnt(sc *sigmaclnt.SigmaClnt, realm sp.Trealm) *NdClnt {
	return &NdClnt{
		scRoot: sc,
		pn:     filepath.Join(sp.REALMS, realm.String()),
	}
}

func (ndc *NdClnt) PathName() string {
	return ndc.pn
}

// Remove the named ep file for the new realm, in case the realm name
// is re-used and it hasn't been clean removed yet (e.g., by
// realmd). Once it is removed, we can watch for it to detect if the
// named of the new incarnation of the realm exists. (The EP file
// isn't leased and thus not automatically deleted.)
func (ndc *NdClnt) RemoveNamedEP() error {
	err := ndc.scRoot.Remove(ndc.pn)
	return err
}

// Wait until the realm's named has registered its endpoint and is
// ready to serve
func (ndc *NdClnt) WaitNamed() error {
	if b, err := ndc.scRoot.GetFileWatch(ndc.pn); err != nil {
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
	if err := ndc.scRoot.Spawn(nd); err != nil {
		return err
	}
	if err := ndc.scRoot.WaitStart(nd.GetPid()); err != nil {
		return err
	}
	if err := ndc.WaitNamed(); err != nil {
		return err
	}
	db.DPrintf(db.TEST, "New named ready to serve")
	return nil
}

func (ndc *NdClnt) StartNamed(nd *proc.Proc) error {
	if err := ndc.scRoot.Spawn(nd); err != nil {
		return err
	}
	if err := ndc.scRoot.WaitStart(nd.GetPid()); err != nil {
		return err
	}
	db.DPrintf(db.TEST, "New named spawned")
	return nil
}

func (ndc *NdClnt) StopNamed(nd *proc.Proc) error {
	// Evict the named
	if err := ndc.scRoot.Evict(nd.GetPid()); err != nil {
		return err
	}
	status, err := ndc.scRoot.WaitExit(nd.GetPid())
	if err != nil {
		return err
	}
	if !status.IsStatusEvicted() {
		return fmt.Errorf("Wrong exit status %v", status)
	}
	return err
}
