package ndclnt

import (
	"fmt"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
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

func StartNamed(sc *sigmaclnt.SigmaClnt, nd *proc.Proc) error {
	// Spawn the named proc
	if err := sc.Spawn(nd); err != nil {
		return err
	}
	// Wait for the proc to start
	if err := sc.WaitStart(nd.GetPid()); err != nil {
		return err
	}
	// Wait for the named to start up
	pn := path.MarkResolve(filepath.Join(sp.REALMS, test.REALM1.String()))
	_, err := sc.GetFileWatch(pn)
	// wait until the named has registered its endpoint and is ready to serve
	if err != nil {
		return err
	}
	db.DPrintf(db.TEST, "New named ready to serve")
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
