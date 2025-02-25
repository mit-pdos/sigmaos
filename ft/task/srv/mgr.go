package srv

import (
	"sigmaos/ft/procgroupmgr"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type FtTaskSrvMgr struct {
	sc *sigmaclnt.SigmaClnt
	Id task.FtTaskSrvId
	p *procgroupmgr.ProcGroupMgr
}

func NewFtTaskSrvMgr(sc *sigmaclnt.SigmaClnt, id string) (*FtTaskSrvMgr, error) {
	err := sc.MkDir(sp.FTTASK, 0777)
	if err != nil && !serr.IsErrorExists(err) {
		return nil, err
	}

	config := procgroupmgr.NewProcGroupConfig(1, "fttask", []string{id}, 0, id)
	p := config.StartGrpMgr(sc)
	err = p.WaitStart()
	if err != nil {
		return nil, err
	}

	return &FtTaskSrvMgr{sc, task.FtTaskSrvId(id), p}, nil
}

func (ft *FtTaskSrvMgr) Stop() error {
	// lock to ensure it doesn't change while we clear the db
	ft.p.Lock()
	clnt := fttask_clnt.NewFtTaskClnt[any, any](ft.sc.FsLib, ft.Id)
	clnt.ClearEtcd()
	ft.p.Unlock()
	_, err := ft.p.StopGroup()
	return err
}
