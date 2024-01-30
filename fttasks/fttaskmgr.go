package fttasks

import (
	"strings"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type FtTaskMgr struct {
	*FtTasks
	proc.ProcAPI
	ntask int32
}

type Tresult struct {
	t      string
	ok     bool
	ms     int64
	status *proc.Status
}

type Ttask struct {
	name string
	fn   string
}

func NewTaskMgr(pclnt proc.ProcAPI, ft *FtTasks) (*FtTaskMgr, error) {
	if err := ft.RecoverTasks(); err != nil {
		return nil, err
	}
	return &FtTaskMgr{ProcAPI: pclnt, FtTasks: ft}, nil
}

func (ftm *FtTaskMgr) ExecuteTasks(mkProc func(n string) *proc.Proc) *proc.Status {
	ch := make(chan Tresult)
	finish := make(chan bool)
	res := make(chan *Tresult)

	go ftm.collector(ch, finish, res)

	// keep doing work until collector tells us to stop (e.g., because
	// unrecoverable error) or until a client stops ftm.
	stop := false
	for !stop {
		sts, err := ftm.WaitForTasks()
		if err != nil {
			db.DFatalf("WaitForTasks %v err %v", err)
		}
		stop = ftm.doTasks(sts, ch, mkProc)
	}
	// tell collector to finish up
	finish <- true
	if r := <-res; r != nil {
		return r.status
	}
	return nil
}

func (ftm *FtTaskMgr) doTasks(sts []*sp.Stat, ch chan Tresult, mkProc func(n string) *proc.Proc) bool {
	// Due to inconsistent views of the WIP directory (concurrent adds by
	// clients and paging reads in the parent of this function), some
	// entries may be duplicated.
	entries := make(map[string]bool)
	for _, st := range sts {
		entries[st.Name] = true
	}
	db.DPrintf(db.FTTASKMGR, "Removed %v duplicate entries", len(sts)-len(entries))
	stop := false
	ntask := 0
	for entry, _ := range entries {
		t, err := ftm.ClaimTask(entry)
		if err != nil || t == "" {
			continue
		}
		s3fn, err := ftm.ReadTask(t)
		if err != nil {
			continue
		}
		if string(s3fn) == STOP {
			// stop after processing remaining entries
			stop = true
			continue
		}
		inputs := strings.Split(string(s3fn), ",")
		for _, input := range inputs {
			atomic.AddInt32(&ftm.ntask, 1)
			ntask += 1
			// Run the task in another thread.
			t := &Ttask{t, input}
			p := mkProc(t.fn)
			go ftm.runTask(p, t, ch)
		}
	}
	db.DPrintf(db.FTTASKMGR, "Started %v tasks stop %v ntask in progress %v", ntask, stop, atomic.LoadInt32(&ftm.ntask))
	return stop
}

func (ftm *FtTaskMgr) runTask(p *proc.Proc, t *Ttask, ch chan Tresult) {
	db.DPrintf(db.FTTASKMGR, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	// Spawn proc.
	err := ftm.Spawn(p)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Couldn't spawn a task %v, err: %v", t, err)
		ch <- Tresult{t.name, false, 0, nil}
	} else {
		db.DPrintf(db.FTTASKMGR, "spawned task %v %v", p.GetPid(), p.Args)
		res := ftm.waitForTask(start, p, t)
		ch <- res
	}
}

func (ftm *FtTaskMgr) waitForTask(start time.Time, p *proc.Proc, t *Ttask) Tresult {
	ftm.WaitStart(p.GetPid())
	db.DPrintf(db.ALWAYS, "Start Latency %v", time.Since(start))
	status, err := ftm.WaitExit(p.GetPid())
	ms := time.Since(start).Milliseconds()
	if err == nil && status.IsStatusOK() {
		if err := ftm.MarkDone(t.name); err != nil {
			db.DFatalf("MarkDone %v done err %v", t.name, err)
		}
		return Tresult{t.name, true, ms, status}
	} else if err == nil && status.IsStatusErr() {
		db.DPrintf(db.ALWAYS, "task %v errored err %v", t, status)
		// mark task as done, but return error
		if err := ftm.MarkDone(t.name); err != nil {
			db.DFatalf("MarkDone %v done err %v", t.name, err)
		}
		return Tresult{t.name, false, ms, status}
	} else { // task failed; make it runnable again
		db.DPrintf(db.FTTASKMGR, "task %v failed %v err %v", t, status, err)
		if err := ftm.MarkRunnable(t.name); err != nil {
			db.DFatalf("MarkRunnable %v err %v", t, err)
		}
		return Tresult{t.name, false, ms, nil}
	}
}

func (ftm *FtTaskMgr) collector(ch chan Tresult, finish chan bool, res chan *Tresult) {
	var r *Tresult
	stop := false
	for !stop || atomic.LoadInt32(&ftm.ntask) > 0 {
		select {
		case <-finish:
			stop = true
		case res := <-ch:
			atomic.AddInt32(&ftm.ntask, -1)
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
