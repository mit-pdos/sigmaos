package task_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	fttask "sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_mgr "sigmaos/ft/task/mgr"
	"sigmaos/test"
	rd "sigmaos/util/rand"
)

const (
	TASKS = "name/tasks"
)

func TestCompile(t *testing.T) {
}

type Tstate struct {
	job string
	*test.Tstate
	ft *fttask.FtTasks
}

func newTstate(t *testing.T) (*Tstate, error) {
	ts1, err1 := test.NewTstate(t)
	if err1 != nil {
		return nil, err1
	}
	ts := &Tstate{Tstate: ts1, job: rd.String(4)}
	ft, err := fttask.MkFtTasks(ts.SigmaClnt.FsLib, TASKS, ts.job)
	if !assert.Nil(ts.T, err) {
		return nil, err
	}
	ts.ft = ft
	return ts, nil
}

func (ts *Tstate) shutdown() {
	ts.RmDir(TASKS)
	ts.Shutdown()
}

func TestBasic(t *testing.T) {
	ts, err1 := newTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	bin := make(mr.Bin, 1)
	bin[0].File = "hello"

	err := ts.ft.SubmitTask(0, bin)
	assert.Nil(t, err)
	tns, err := ts.ft.AcquireTasks()
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "Tasks %v", tns)
	s, err := ts.ft.JobState()
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "JobState %v", s)
	var b mr.Bin
	err = ts.ft.ReadTask(tns[0], &b)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "Task %v", b)
	err = ts.ft.MarkDoneOutput(tns[0], "bye")
	assert.Nil(t, err)
	s, err = ts.ft.JobState()
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "JobState %v", s)
	err = ts.ft.ReadTaskOutput(tns[0], &b)
	db.DPrintf(db.TEST, "Output %v", b)
	ts.shutdown()
}

func TestStats(t *testing.T) {
	ts, err1 := newTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	bin := make(mr.Bin, 1)
	bin[0].File = "hello"

	err := ts.ft.SubmitTask(0, bin)
	assert.Nil(t, err)

	sts := ts.ft.GetStats()
	assert.Equal(t, 1, sts.Ntask)
	_, err = ts.ft.AcquireTasks()
	assert.Nil(t, err)

	ft, err := fttask.NewFtTasks(ts.FsLib, TASKS, ts.job)
	assert.Nil(t, err)
	_, err = ft.RecoverTasks()
	assert.Nil(t, err)

	sts = ft.GetStats()
	assert.Equal(t, 2, sts.Ntask)

	ft, err = fttask.NewFtTasks(ts.FsLib, TASKS, ts.job)
	assert.Nil(t, err)
	sts = ft.GetStats()
	assert.Equal(t, 2, sts.Ntask)

	ts.shutdown()
}

func TestPerf(t *testing.T) {
	nTasks := 1000

	ts, err1 := newTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	bin := make(mr.Bin, 1)
	bin[0].File = "hello"

	start := time.Now()
	for i := 0; i < nTasks; i++ {
		err := ts.ft.SubmitTask(0, bin)
		assert.Nil(t, err)
	}
	db.DPrintf(db.ALWAYS, "Submitting tasks took %v (%v per task)", time.Since(start), time.Since(start) / time.Duration(nTasks))

	start = time.Now()
	tns, err := ts.ft.AcquireTasks()
	db.DPrintf(db.ALWAYS, "Acquired tasks in %v (%v per task)", time.Since(start), time.Since(start) / time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(tns))

	start = time.Now()
	for _, tn := range tns {
		var b mr.Bin
		err = ts.ft.ReadTask(tn, &b)
		assert.Nil(t, err)
		assert.Equal(t, bin, b)
	}
	db.DPrintf(db.ALWAYS, "Read tasks in %v (%v per task)", time.Since(start), time.Since(start) / time.Duration(nTasks))

	start = time.Now()
	for _, tn := range tns {
		err = ts.ft.MarkDoneOutput(tn, "bye")
		assert.Nil(t, err)
	}
	db.DPrintf(db.ALWAYS, "Marked all tasks done in %v (%v per task)", time.Since(start), time.Since(start) / time.Duration(nTasks))

	start = time.Now()
	for _, tn := range tns {
		var output string
		err = ts.ft.ReadTaskOutput(tn, &output)
		assert.Nil(t, err)
		assert.Equal(t, "bye", output)
	}
	db.DPrintf(db.ALWAYS, "Read all outputs in %v (%v per task)", time.Since(start), time.Since(start) / time.Duration(nTasks))

	ts.shutdown()
}

func TestRPC(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	mgr, err := fttask_mgr.NewFtTaskSrvMgr(ts.SigmaClnt, "test")
	assert.Nil(t, err)

	clnt := fttask_clnt.NewTaskClnt(ts.FsLib, mgr.Id)
	resp, err := clnt.Echo("hello")
	assert.Nil(t, err)
	assert.Equal(t, "hello", resp)

	mgr.Close()

	ts.Shutdown()
}
