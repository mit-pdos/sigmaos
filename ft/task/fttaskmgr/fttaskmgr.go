// The ftttaskmgr package executes tasks as procs and returns the
// results to client through a Tresult channel.
package fttaskmgr

import (
	"time"

	procapi "sigmaos/api/proc"
	db "sigmaos/debug"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/ft/task/proto"
	"sigmaos/proc"
)

type Tresult[Data any, Output any] struct {
	Ms     time.Duration
	Err    error
	Status *proc.Status
	Proc   *proc.Proc
	Id     fttask_clnt.TaskId
	Ftclnt fttask_clnt.FtTaskClnt[Data, Output]
}

type FtTaskCoord[Data any, Output any] struct {
	ftclnt   fttask_clnt.FtTaskClnt[Data, Output]
	pclnt    procapi.ProcAPI
	chResult chan<- Tresult[Data, Output]
}

type TnewProc[Data any] func(fttask_clnt.Task[Data]) (*proc.Proc, error)

func NewFtTaskCoord[Data any, Output any](pclnt procapi.ProcAPI, ft fttask_clnt.FtTaskClnt[Data, Output], ch chan<- Tresult[Data, Output]) (*FtTaskCoord[Data, Output], error) {
	return &FtTaskCoord[Data, Output]{
		pclnt:    pclnt,
		ftclnt:   ft,
		chResult: ch,
	}, nil
}

func (ftm *FtTaskCoord[Data, Output]) ExecuteTasks(mkProc TnewProc[Data]) (*proto.TaskStats, error) {
	chTask := make(chan []fttask_clnt.TaskId)

	go fttask_clnt.GetTasks(ftm.ftclnt, chTask)

	for tasks := range chTask {
		if err := ftm.startTasks(tasks, mkProc); err != nil {
			db.DPrintf(db.FTTASKMGR, "ExecuteTasks: startTasks err %v", err)
			return nil, err
		}
	}
	stats, err := ftm.ftclnt.Stats()
	if err != nil {
		db.DPrintf(db.FTTASKMGR, "ExecuteTasks: Stats err %v", err)
		return nil, err
	}
	db.DPrintf(db.FTTASKMGR, "ExecuteTasks: done %v", stats)
	return stats, nil
}

func (ftm *FtTaskCoord[Data, Output]) startTasks(ids []fttask_clnt.TaskId, newProc TnewProc[Data]) error {
	db.DPrintf(db.FTTASKMGR, "runTasks %v", len(ids))
	start := time.Now()
	tasks, err := ftm.ftclnt.ReadTasks(ids)
	if err != nil {
		db.DPrintf(db.FTTASKMGR, "ReadTasks %v err %v", tasks, err)
		return err
	}
	db.DPrintf(db.FTTASKMGR, "runTasks: read tasks %v time: %v", len(tasks), time.Since(start))

	// create all proc objects first so we can spawn them all in
	// quick succession to try to balance load across machines
	procs := make([]*proc.Proc, len(tasks))
	for i, t := range tasks {
		proc, err := newProc(t)
		if err != nil {
			db.DFatalf("Err spawn task: %v", err)
		}
		procs[i] = proc
	}

	for i, task := range tasks {
		go ftm.runTask(procs[i], task.Id)
	}

	db.DPrintf(db.FTTASKMGR, "Started %v tasks", len(procs))
	return nil
}

func (ftm *FtTaskCoord[Data, Output]) runTask(p *proc.Proc, t fttask_clnt.TaskId) {
	db.DPrintf(db.FTTASKMGR, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	err := ftm.pclnt.Spawn(p)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Couldn't spawn a task %v, err: %v", t, err)
		if err := ftm.ftclnt.MoveTasks([]fttask_clnt.TaskId{t}, fttask_clnt.TODO); err != nil {
			db.DFatalf("MoveTasks %v TODO err %v", t, err)
		}
	} else {
		db.DPrintf(db.FTTASKMGR, "spawned task %v %v", p.GetPid(), p.Args)
		ftm.waitForTask(start, p, t)
	}
}

func (ftm *FtTaskCoord[Data, Output]) waitForTask(start time.Time, p *proc.Proc, id fttask_clnt.TaskId) {
	ftm.pclnt.WaitStart(p.GetPid())
	db.DPrintf(db.FTTASKMGR, "Start Latency %v", time.Since(start))
	status, err := ftm.pclnt.WaitExit(p.GetPid())
	ms := time.Since(start)
	ftm.chResult <- Tresult[Data, Output]{
		Ms:     ms,
		Err:    err,
		Status: status,
		Proc:   p,
		Id:     id,
		Ftclnt: ftm.ftclnt,
	}
}
