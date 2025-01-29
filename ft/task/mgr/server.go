package fttaskmgr

import (
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type FtTaskSrvMgr struct {
	*sigmaclnt.SigmaClnt
	Id string
	p *proc.Proc
}

func NewFtTaskSrvMgr(sc *sigmaclnt.SigmaClnt, id string) (*FtTaskSrvMgr, error) {
	err := sc.MkDir(sp.FTTASK, 0777)
	if err != nil && !serr.IsErrorExists(err) {
		return nil, err
	}

	p := proc.NewProc("fttask", []string{id})

	if err := sc.Spawn(p); err != nil {
		return nil, err
	}
	if err := sc.WaitStart(p.GetPid()); err != nil {
		return nil, err
	}
	return &FtTaskSrvMgr{sc, id, p}, nil
}

func (ft *FtTaskSrvMgr) Stop() error {
	if err := ft.Evict(ft.p.GetPid()); err != nil {
		return err
	}

	_, err := ft.SigmaClnt.WaitExit(ft.p.GetPid())
	return err
}
