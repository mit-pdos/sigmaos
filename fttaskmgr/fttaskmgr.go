// The fttaskmgr implements a task manager using [fttasks], which
// stores tasks persistently.  The manger proc spawns procs to process
// these tasks, and restarts them if a proc crashes.  The fttask mgr
// itself is fault-tolerant: after a crash, another mgr procs will
// take over and resumes from the fttask state. [imgrsizesrv] uses
// [fttaskmgr] to proces image-resizing tasks.
package fttaskmgr

import (
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/fttasks"
	"sigmaos/proc"
)

type FtTaskMgr struct {
	*fttasks.FtTasks
	proc.ProcAPI
	ntask atomic.Int32
}

type Tresult struct {
	t      string
	ok     bool
	ms     int64
	status *proc.Status
}

type Tnew func() interface{}
type TmkProc func(n string, i interface{}) *proc.Proc

func NewTaskMgr(pclnt proc.ProcAPI, ft *fttasks.FtTasks) (*FtTaskMgr, error) {
	if err := ft.RecoverTasks(); err != nil {
		return nil, err
	}
	return &FtTaskMgr{ProcAPI: pclnt, FtTasks: ft}, nil
}

func (ftm *FtTaskMgr) ExecuteTasks(new Tnew, mkProc TmkProc) *proc.Status {
	ch := make(chan Tresult)
	finish := make(chan bool)
	res := make(chan *Tresult)

	go ftm.collector(ch, finish, res)

	// keep doing work until collector tells us to stop (e.g., because
	// unrecoverable error) or until a client stops ftm.
	stop := false
	for !stop {
		ts, err := ftm.WaitForTasks()
		if err != nil {
			db.DFatalf("WaitForTasks err %v", err)
		}
		b, err := ftm.StartTasks(ts, ch, new, mkProc)
		if err != nil {
			db.DFatalf("startTasks %v err %v", ts, err)
		}
		stop = b
	}
	// tell collector to finish up
	finish <- true
	if r := <-res; r != nil {
		return r.status
	}
	return nil
}

func (ftm *FtTaskMgr) StartTasks(ts []string, ch chan Tresult, new Tnew, mkProc TmkProc) (bool, error) {
	ntask := 0
	var r error
	stop := false
	for _, t := range ts {
		if t == fttasks.STOP {
			db.DPrintf(db.FTTASKMGR, "StartTasks stop %v", t)
			stop = true
			continue
		}
		rdr, err := ftm.TaskReader(t)
		if err != nil {
			db.DPrintf(db.FTTASKMGR, "TaskReader %s err %v", t, err)
			r = err
			continue
		}
		defer rdr.Close()
		err = fslib.JsonReader(rdr.Reader, new, func(i interface{}) error {
			ftm.ntask.Add(1)
			ntask += 1
			p := mkProc(t, i)
			// Run the task in another thread.
			go ftm.runTask(p, t, ch)
			return nil
		})
	}
	db.DPrintf(db.FTTASKMGR, "Started %v tasks ntask in progress %v %v", ntask, ftm.ntask.Load(), stop)
	return stop, r
}

func (ftm *FtTaskMgr) runTask(p *proc.Proc, t string, ch chan Tresult) {
	db.DPrintf(db.FTTASKMGR, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	// Spawn proc.
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

func (ftm *FtTaskMgr) waitForTask(start time.Time, p *proc.Proc, t string) Tresult {
	ftm.WaitStart(p.GetPid())
	db.DPrintf(db.ALWAYS, "Start Latency %v", time.Since(start))
	status, err := ftm.WaitExit(p.GetPid())
	ms := time.Since(start).Milliseconds()
	if err == nil && status.IsStatusOK() {
		if err := ftm.MarkDone(t); err != nil {
			db.DFatalf("MarkDone %v done err %v", t, err)
		}
		return Tresult{t, true, ms, status}
	} else if err == nil && status.IsStatusErr() {
		db.DPrintf(db.ALWAYS, "task %v errored err %v", t, status)
		// mark task as done, but return error
		if err := ftm.MarkDone(t); err != nil {
			db.DFatalf("MarkDone %v done err %v", t, err)
		}
		return Tresult{t, false, ms, status}
	} else { // task failed; make it runnable again
		db.DPrintf(db.FTTASKMGR, "task %v failed %v err %v", t, status, err)
		if err := ftm.MarkRunnable(t); err != nil {
			db.DFatalf("MarkRunnable %v err %v", t, err)
		}
		return Tresult{t, false, ms, nil}
	}
}

func (ftm *FtTaskMgr) collector(ch chan Tresult, finish chan bool, res chan *Tresult) {
	var r *Tresult
	stop := false
	for !stop || ftm.ntask.Load() > 0 {
		select {
		case <-finish:
			stop = true
		case res := <-ch:
			ftm.ntask.Add(-1)
			if res.ok {
				db.DPrintf(db.FTTASKMGR, "%v ok %v ms %d msg %v", res.t, res.ok, res.ms, res.status)
			}
			if !res.ok && res.status != nil {
				db.DPrintf(db.ALWAYS, "task %v has unrecoverable err %v\n", res.t, res.status)
				r = &res
			}
		}
	}
	res <- r
}
