package srv

import (
	"sigmaos/ft/procgroupmgr"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
)

type FtTaskSrvMgr struct {
	sc *sigmaclnt.SigmaClnt
	Id task.FtTaskSrvId
	p *procgroupmgr.ProcGroupMgr
}

func NewFtTaskSrvMgr(sc *sigmaclnt.SigmaClnt, id string, em *crash.TeventMap) (*FtTaskSrvMgr, error) {
	err := sc.MkDir(sp.FTTASK, 0777)
	if err != nil && !serr.IsErrorExists(err) {
		return nil, err
	}

	err = crash.SetSigmaFail(em)
	if err != nil {
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

func (ft *FtTaskSrvMgr) Stop(clearStore bool) error {
	if clearStore {
		// lock to ensure group members don't change while we clear the db
		ft.p.Lock()
		clnt := fttask_clnt.NewFtTaskClnt[any, any](ft.sc.FsLib, ft.Id)
		err := clnt.ClearEtcd()
		ft.p.Unlock()
		if err != nil {
			return err
		}
	}
	_, err := ft.p.StopGroup()
	return err
}
