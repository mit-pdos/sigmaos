package fttaskmgr

import (
	"time"

	procapi "sigmaos/api/proc"
	db "sigmaos/debug"
	ftclnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	"sigmaos/util/spstats"
)

type AStat struct {
	Nok    spstats.Tcounter
	Nerror spstats.Tcounter
	Nfail  spstats.Tcounter
}

type FtTaskCoord[Data any, Output any] struct {
	ftclnt.FtTaskClnt[Data, Output]
	procapi.ProcAPI
	AStat
}

type Tnew[Data any] func() Data
type TmkProc[Data any] func(ftclnt.Task[Data]) *proc.Proc

func NewFtTaskCoord[Data any, Output any](pclnt procapi.ProcAPI, ft ftclnt.FtTaskClnt[Data, Output]) (*FtTaskCoord[Data, Output], error) {
	if _, err := ft.MoveTasksByStatus(ftclnt.WIP, ftclnt.TODO); err != nil {
		return nil, err
	}
	return &FtTaskCoord[Data, Output]{
		ProcAPI:    pclnt,
		FtTaskClnt: ft,
	}, nil
}

func (ftm *FtTaskCoord[Data, Output]) ExecuteTasks(mkProc TmkProc[Data]) *spstats.TcounterSnapshot {
	chTask := make(chan []ftclnt.TaskId)

	go ftclnt.GetTasks(ftm, chTask)

	for tasks := range chTask {
		err := ftm.startTasks(tasks, mkProc)
		if err != nil {
			db.DFatalf("StartTasks %v err %v", tasks, err)
		}
	}
	db.DPrintf(db.FTTASKMGR, "ExecuteTasks: done %v", ftm.AStat)
	stro := spstats.NewTcounterSnapshot()
	stro.FillCounters(&ftm.AStat)
	return stro
}

func (ftm *FtTaskCoord[Data, Output]) startTasks(tasks []ftclnt.TaskId, mkProc TmkProc[Data]) error {
	ntask := 0
	tasksData, err := ftm.ReadTasks(tasks)
	if err != nil {
		db.DPrintf(db.FTTASKMGR, "ReadTasks %v err %v", tasks, err)
		return err
	}

	for _, task := range tasksData {
		ntask += 1
		p := mkProc(task)
		go ftm.runTask(p, task.Id)
	}

	db.DPrintf(db.FTTASKMGR, "Started %v tasks", ntask)
	return nil
}

func (ftm *FtTaskCoord[Data, Output]) runTask(p *proc.Proc, t ftclnt.TaskId) {
	db.DPrintf(db.FTTASKMGR, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	err := ftm.Spawn(p)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Couldn't spawn a task %v, err: %v", t, err)
		if err := ftm.MoveTasks([]ftclnt.TaskId{t}, ftclnt.TODO); err != nil {
			db.DFatalf("MoveTasks %v TODO err %v", t, err)
		}
	} else {
		db.DPrintf(db.FTTASKMGR, "spawned task %v %v", p.GetPid(), p.Args)
		ftm.waitForTask(start, p, t)
	}
}

func (ftm *FtTaskCoord[Data, Output]) waitForTask(start time.Time, p *proc.Proc, id ftclnt.TaskId) {
	ftm.WaitStart(p.GetPid())
	db.DPrintf(db.ALWAYS, "Start Latency %v", time.Since(start))
	status, err := ftm.WaitExit(p.GetPid())
	if err == nil && status.IsStatusOK() {
		spstats.Inc(&ftm.AStat.Nok, 1)
		if err := ftm.MoveTasks([]ftclnt.TaskId{id}, ftclnt.DONE); err != nil {
			db.DFatalf("MoveTasks %v done err %v", id, err)
		}
	} else if err == nil && status.IsStatusErr() && !status.IsCrashed() {
		db.DPrintf(db.ALWAYS, "task %v errored status %v msg %v", id, status, status.Msg())
		spstats.Inc(&ftm.AStat.Nerror, 1)
		// mark task as done, but return error
		if err := ftm.MoveTasks([]ftclnt.TaskId{id}, ftclnt.ERROR); err != nil {
			db.DFatalf("MoveTasks %v error err %v", id, err)
		}
		// XXX write status to output
		//if err := ftm.AddTaskOutputs([]ftclnt.TaskId{id}, status, false); err != nil {
		//	db.DFatalf("AddTaskOutputs %v error err %v", id, err)
		//}
	} else { // an error, task crashed, or was evicted; make it runnable again
		db.DPrintf(db.FTTASKMGR, "task %v failed %v err %v msg %q", id, status, err, status.Msg())
		spstats.Inc(&ftm.AStat.Nfail, 1)
		if err := ftm.MoveTasks([]ftclnt.TaskId{id}, ftclnt.TODO); err != nil {
			db.DFatalf("MoveTasks %v todo err %v", id, err)
		}
	}
}
