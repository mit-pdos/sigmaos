package srv

import (
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type FtTaskSrvId string

type FtTaskSrvMgr struct {
	sc *sigmaclnt.SigmaClnt
	Id FtTaskSrvId
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
	return &FtTaskSrvMgr{sc, FtTaskSrvId(id), p}, nil
}

func (ft *FtTaskSrvMgr) Stop() error {
	if err := ft.sc.Evict(ft.p.GetPid()); err != nil {
		return err
	}

	_, err := ft.sc.WaitExit(ft.p.GetPid())
	return err
}
