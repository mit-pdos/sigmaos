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
	Id      fttask.FtTaskSrvId
	clnt    fttask_clnt.FtTaskClnt[any, any]
	p       *procgroupmgr.ProcGroupMgr
}

// when testing partitions, we don't want to evict unresponsive instances
// to test if new instances can coexist with old ones
func NewFtTaskSrvMgr(sc *sigmaclnt.SigmaClnt, id string, persist bool) (*FtTaskSrvMgr, error) {
	err := sc.MkDir(sp.FTTASK, 0777)
	if err != nil && !serr.IsErrorExists(err) {
		return nil, err
	}

	config := procgroupmgr.NewProcGroupConfig(1, "fttask-srv", []string{}, FTTASK_SRV_MCPU, id)
	if persist {
		config.Persist(sc.FsLib)
	}

	p := config.StartGrpMgr(sc)
	err = p.WaitStart()
	if err != nil {
		return nil, err
	}

	clnt := fttask_clnt.NewFtTaskClnt[any, any](sc.FsLib, fttask.FtTaskSrvId(id))

	ft := &FtTaskSrvMgr{sc, false, fttask.FtTaskSrvId(id), clnt, p}

	return ft, nil
}

func (ft *FtTaskSrvMgr) Stop(clearStore bool) ([]*procgroupmgr.ProcStatus, error) {
	ft.stopped = true
	if clearStore {
		db.DPrintf(db.FTTASKS, "Sending request to clear backing store")
		// lock to ensure group members don't change while we clear the db
		ft.p.Lock()
		err := ft.clnt.ClearEtcd()
		ft.p.Unlock()
		if err != nil {
			return nil, err
		}
	}
	db.DPrintf(db.FTTASKS, "Stopping group %v", ft.Id)
	stats, err := ft.p.StopGroup()
	return stats, err
}
