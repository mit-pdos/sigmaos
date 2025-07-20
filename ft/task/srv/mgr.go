// Handles the creation and auto-restart of a fault tolerant task server
// using [ft/procgroupmgr].
package srv

import (
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	fttask "sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const FTTASK_SRV_MCPU proc.Tmcpu = 1000

type FtTaskSrvMgr struct {
	sc      *sigmaclnt.SigmaClnt
	stopped bool
	Id      fttask.FtTaskSvcId
	clnt    fttask_clnt.FtTaskClnt[any, any]
	pgm     *procgroupmgr.ProcGroupMgr
}

func NewFtTaskSrvMgr(sc *sigmaclnt.SigmaClnt, id string, persist bool) (*FtTaskSrvMgr, error) {
	err := sc.MkDir(sp.FTTASK, 0777)
	if err != nil && !serr.IsErrorExists(err) {
		return nil, err
	}

	config := procgroupmgr.NewProcGroupConfig(1, "fttask-srv", []string{}, FTTASK_SRV_MCPU, id)
	if persist {
		config.Persist(sc.FsLib)
	}

	pgm := config.StartGrpMgr(sc)

	clnt := fttask_clnt.NewFtTaskClnt[any, any](sc.FsLib, fttask.FtTaskSvcId(id))

	ft := &FtTaskSrvMgr{sc, false, fttask.FtTaskSvcId(id), clnt, pgm}

	return ft, nil
}

func (ft *FtTaskSrvMgr) Stop(clearStore bool) ([]*procgroupmgr.ProcStatus, error) {
	ft.stopped = true
	if clearStore {
		db.DPrintf(db.FTTASKMGR, "Sending request to clear backing store")
		// XXX lock to ensure group members don't change while we clear the db
		err := ft.clnt.ClearEtcd()
		if err != nil {
			return nil, err
		}
	}
	db.DPrintf(db.FTTASKMGR, "Stopping group %v", ft.Id)
	stats, err := ft.pgm.StopGroup()
	return stats, err
}
