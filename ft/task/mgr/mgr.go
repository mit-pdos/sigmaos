package fttaskmgr

import (
	procapi "sigmaos/api/proc"
	db "sigmaos/debug"
	ftclnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	"sync/atomic"
	"time"
)

type FtTaskMgr[Data any, Output any] struct {
	ftclnt.FtTaskClnt[Data, Output]
	procapi.ProcAPI
	nTasksRunning atomic.Int32
}

type Tresult struct {
	id     ftclnt.TaskId
	ok     bool
	ms     int64
	status *proc.Status
}

type Tnew[Data any] func() Data
type TmkProc[Data any] func(ftclnt.Task[Data]) *proc.Proc

func NewTaskMgr[Data any, Output any](pclnt procapi.ProcAPI, ft ftclnt.FtTaskClnt[Data, Output]) (*FtTaskMgr[Data, Output], error) {
	if _, err := ft.MoveTasksByStatus(ftclnt.WIP, ftclnt.TODO); err != nil {
		return nil, err
	}

	return &FtTaskMgr[Data, Output]{ProcAPI: pclnt, FtTaskClnt: ft}, nil
}

func (ftm *FtTaskMgr[Data, Output]) ExecuteTasks(mkProc TmkProc[Data]) *proc.Status {
	var r *Tresult
	chRes := make(chan Tresult)
	chTask := make(chan []ftclnt.TaskId)
	chStop := make(chan bool)

	go ftm.getTasks(chTask, chStop)

	// keep doing work until startTasks us to stop (e.g., clients
	// stops ftm) or unrecoverable error.
	stopped := false
	receivedStop := false
	for !stopped {
		select {
		case res := <-chRes:
			ftm.nTasksRunning.Add(-1)
			if ftm.nTasksRunning.Load() == 0 {
				n, err := ftm.GetNTasks(ftclnt.TODO)
				if err != nil {
					db.DFatalf("GetNTasks err %v", err)
				}
				db.DPrintf(db.FTTASKMGR, "receivedStop %t ntask=0 n %d", receivedStop, n)
				if n == 0 && receivedStop {
					stopped = true
				}
			}
			if res.ok {
				db.DPrintf(db.FTTASKMGR, "%v ok %v ms %d msg %v", res.id, res.ok, res.ms, res.status)
			} else if res.status != nil {
				db.DPrintf(db.ALWAYS, "task %v has unrecoverable err %v\n", res.id, res.status)
				r = &res
				break
			}
		case tasks := <-chTask:
			err := ftm.startTasks(tasks, chRes, mkProc)
			if err != nil {
				db.DFatalf("StartTasks %v err %v", tasks, err)
			}
		case s := <-chStop:
			receivedStop = s
		}
	}

	db.DPrintf(db.FTTASKMGR, "ExecuteTasks: done")
	if r != nil {
		return r.status
	}
	return nil
}

func (ftm *FtTaskMgr[Data, Output]) getTasks(chTask chan<- []ftclnt.TaskId, chStop chan<- bool) {
	for {
		tasks, stopped, err := ftm.AcquireTasks(true)
		if err != nil {
			db.DFatalf("AcquireTasks err %v", err)
		}

		if stopped {
			chStop <- true
			break
		}

		if len(tasks) != 0 {
			chTask <- tasks
		}
	}
}

func (ftm *FtTaskMgr[Data, Output]) startTasks(tasks []ftclnt.TaskId, ch chan Tresult, mkProc TmkProc[Data]) error {
	ntask := 0
	tasksData, err := ftm.ReadTasks(tasks)
	if err != nil {
		db.DPrintf(db.FTTASKMGR, "ReadTasks %v err %v", tasks, err)
		return err
	}

	for _, task := range tasksData {
		ftm.nTasksRunning.Add(1)
		ntask += 1
		p := mkProc(task)
		go ftm.runTask(p, task.Id, ch)
	}

	db.DPrintf(db.FTTASKMGR, "Started %v tasks ntask in progress %v", ntask, ftm.nTasksRunning.Load())
	return nil
}

func (ftm *FtTaskMgr[Data, Output]) runTask(p *proc.Proc, t ftclnt.TaskId, ch chan<- Tresult) {
	db.DPrintf(db.FTTASKMGR, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	err := ftm.Spawn(p)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Couldn't spawn a task %v, err: %v", t, err)
		ch <- Tresult{t, false, 0, nil}
	} else {
		db.DPrintf(db.FTTASKMGR, "spawned task %v %v", p.GetPid(), p.Args)
		res := ftm.waitForTask(start, p, t)
		ch <- res
	}
}

func (ftm *FtTaskMgr[Data, Output]) waitForTask(start time.Time, p *proc.Proc, id ftclnt.TaskId) Tresult {
	ftm.WaitStart(p.GetPid())
	db.DPrintf(db.ALWAYS, "Start Latency %v", time.Since(start))
	status, err := ftm.WaitExit(p.GetPid())
	ms := time.Since(start).Milliseconds()
	if err == nil && status.IsStatusOK() {
		if err := ftm.MoveTasks([]ftclnt.TaskId{id}, ftclnt.DONE); err != nil {
			db.DFatalf("MarkDone %v done err %v", id, err)
		}
		return Tresult{id, true, ms, status}
	} else if err == nil && status.IsStatusErr() && !status.IsCrashed() {
		db.DPrintf(db.ALWAYS, "task %v errored status %v msg %v", id, status, status.Msg())
		// mark task as done, but return error
		if err := ftm.MoveTasks([]ftclnt.TaskId{id}, ftclnt.DONE); err != nil {
			db.DFatalf("MarkDone %v done err %v", id, err)
		}
		return Tresult{id, false, ms, status}
	} else { // an error, task crashed, or was evicted; make it runnable again
		db.DPrintf(db.FTTASKMGR, "task %v failed %v err %v msg %q", id, status, err, status.Msg())
		if err := ftm.MoveTasks([]ftclnt.TaskId{id}, ftclnt.TODO); err != nil {
			db.DFatalf("MarkRunnable %v err %v", id, err)
		}
		return Tresult{id, false, ms, nil}
	}
}