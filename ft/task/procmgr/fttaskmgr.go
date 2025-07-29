// The ftttaskmgr package executes tasks as procs and returns the
// results to client through a Tresult channel.
package fttaskmgr

import (
	"time"

	procapi "sigmaos/api/proc"
	db "sigmaos/debug"
	ftclnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type Tresult struct {
	Ms     time.Duration
	Err    error
	Status *proc.Status
	Pid    sp.Tpid
	Id     ftclnt.TaskId
}

type FtTaskCoord[Data any, Output any] struct {
	ftclnt.FtTaskClnt[Data, Output]
	procapi.ProcAPI
	chResult chan<- Tresult
}

type TnewProc[Data any] func(ftclnt.Task[Data]) (*proc.Proc, error)

func NewFtTaskCoord[Data any, Output any](pclnt procapi.ProcAPI, ft ftclnt.FtTaskClnt[Data, Output], ch chan<- Tresult) (*FtTaskCoord[Data, Output], error) {
	return &FtTaskCoord[Data, Output]{
		ProcAPI:    pclnt,
		FtTaskClnt: ft,
		chResult:   ch,
	}, nil
}

func (ftm *FtTaskCoord[Data, Output]) ExecuteTasks(mkProc TnewProc[Data]) {
	chTask := make(chan []ftclnt.TaskId)

	go ftclnt.GetTasks(ftm, chTask)

	for tasks := range chTask {
		err := ftm.startTasks(tasks, mkProc)
		if err != nil {
			db.DFatalf("StartTasks %v err %v", tasks, err)
		}
	}
	db.DPrintf(db.FTTASKMGR, "ExecuteTasks: done")
}

func (ftm *FtTaskCoord[Data, Output]) startTasks(ids []ftclnt.TaskId, newProc TnewProc[Data]) error {
	db.DPrintf(db.FTTASKMGR, "runTasks %v", len(ids))
	start := time.Now()
	tasks, err := ftm.ReadTasks(ids)
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
	db.DPrintf(db.FTTASKMGR, "Start Latency %v", time.Since(start))
	status, err := ftm.WaitExit(p.GetPid())
	ms := time.Since(start)
	ftm.chResult <- Tresult{
		Ms:     ms,
		Err:    err,
		Status: status,
		Pid:    p.GetPid(),
		Id:     id,
	}
}
