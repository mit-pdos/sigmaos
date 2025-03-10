// Handles the creation and auto-restart of a fault tolerant server
package srv

import (
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/ft/task"
	fttask "sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
	"time"
)

type FtTaskSrvMgr struct {
	sc *sigmaclnt.SigmaClnt
	Id task.FtTaskSrvId
	clnt fttask_clnt.FtTaskClnt[any, any]
	p *procgroupmgr.ProcGroupMgr
}

func NewFtTaskSrvMgr(sc *sigmaclnt.SigmaClnt, id string, em *crash.TeventMap) (*FtTaskSrvMgr, error) {
	err := sc.MkDir(sp.FTTASK, 0777)
	if err != nil && !serr.IsErrorExists(err) {
		return nil, err
	}

	err = sc.MkDir(filepath.Join(sp.FTTASK, id), 0777)
	if err != nil && !serr.IsErrorExists(err) {
		return nil, err
	}

	err = crash.SetSigmaFail(em)
	if err != nil {
		return nil, err
	}

	config := procgroupmgr.NewProcGroupConfig(1, "fttask-srv", []string{}, 0, id)
	p := config.StartGrpMgr(sc)
	err = p.WaitStart()
	if err != nil {
		return nil, err
	}

	clnt := fttask_clnt.NewFtTaskClnt[any, any](sc.FsLib, task.FtTaskSrvId(id))

	ft := &FtTaskSrvMgr{sc, task.FtTaskSrvId(id), clnt, p}
	go ft.monitor()

	return ft, nil
}

func (ft *FtTaskSrvMgr) monitor() {
	nfail := 0
	for ft.p.IsRunning() {
		err := ft.clnt.Ping()
		if serr.IsErrorUnavailable(err) {
			if !ft.p.IsRunning() {
				return
			}
			nfail += 1

			if nfail >= fttask.MGR_NUM_FAILS_UNTIL_RESTART {
				db.DPrintf(db.FTTASKS, "Failed to ping server %d times, restarting group", fttask.MGR_NUM_FAILS_UNTIL_RESTART)
				err = ft.p.RestartGroup(true)
				time.Sleep(fttask.MGR_RESTART_TIMEOUT)
				if err != nil {
					db.DPrintf(db.FTTASKS, "Failed to restart group: %v", err)
				}
				nfail = 0
			}
		} else {
			nfail = 0
		}

		time.Sleep(fttask.MGR_PING_TIMEOUT)
	}
}

// for testing a permanent network partition between client
// and currently running instance
// mgr will eventually notice the partition and restart the group
func (ft *FtTaskSrvMgr) Partition() error {
	ft.p.Lock()
	defer ft.p.Unlock()

	// tell server to partition from named and etcd
	currInstance, err := ft.clnt.Partition()
	if err != nil {
		return err
	}

	db.DPrintf(db.FTTASKS, "Partitioning instance %v", currInstance)

	// prevent client from connecting to the partitioned instance
	return ft.sc.Disconnect(filepath.Join(ft.Id.ServerPath(), currInstance))
}

func (ft *FtTaskSrvMgr) Stop(clearStore bool) ([]*procgroupmgr.ProcStatus, error) {
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
	db.DPrintf(db.FTTASKS, "Stopping group")
	stats, err := ft.p.StopGroup()
	return stats, err
}
