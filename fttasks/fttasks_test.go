package fttasks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fttasks"
	"sigmaos/mr"
	rd "sigmaos/rand"
	"sigmaos/test"
)

const (
	TASKS = "name/tasks"
)

func TestCompile(t *testing.T) {
}

type Tstate struct {
	job string
	*test.Tstate
	ft *fttasks.FtTasks
}

func newTstate(t *testing.T) (*Tstate, error) {
	ts1, err1 := test.NewTstate(t)
	if err1 != nil {
		return nil, err1
	}
	ts := &Tstate{Tstate: ts1, job: rd.String(4)}
	ft, err := fttasks.MkFtTasks(ts.SigmaClnt.FsLib, TASKS, ts.job)
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
