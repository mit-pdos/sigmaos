package task_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	fttask "sigmaos/ft/task"
	fttaskclnt "sigmaos/ft/task/clnt"
	"sigmaos/ft/task/proto"
	fttasksrv "sigmaos/ft/task/srv"
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
	assert.Nil(t, err)
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

func testServerContents[Data any, Output any](t *testing.T, clnt *fttaskclnt.FtTaskClnt[Data, Output], todo []int32, wip []int32, done []int32, err []int32) {
	allExpected := [][]int32{todo, wip, done, err}
	statuses := []proto.TaskStatus{proto.TaskStatus_TODO, proto.TaskStatus_WIP, proto.TaskStatus_DONE, proto.TaskStatus_ERROR}

	for ix, status := range statuses {
		expected := allExpected[ix]
		actual, err := clnt.GetTasksByStatus(status)
		assert.Nil(t, err)
		assert.Equal(t, len(expected), len(actual), "expected %v got %v for %v", expected, actual, status)
		for _, num := range expected {
			found := false
			for _, x := range actual {
				if num == x {
					found = true
				}
			}

			assert.True(t, found, "could not find %v in %v", num, status)
		}
	}

	stats, err2 := clnt.Stats()
	assert.Nil(t, err2)
	assert.Equal(t, int32(len(todo)), stats.NumTodo)
	assert.Equal(t, int32(len(wip)),  stats.NumWip)
	assert.Equal(t, int32(len(done)), stats.NumDone)
	assert.Equal(t, int32(len(err)),  stats.NumError)
}

func TestServerMoveTasksByStatus(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	ntasks := 5
	ids := make([]int32, ntasks)
	for i := 0; i < ntasks; i++ {
		ids[i] = int32(i)
	}

	db.DPrintf(db.TEST, "Making fttasks server")
	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test")
	assert.Nil(t, err)

	clnt := fttaskclnt.NewFtTaskClnt[struct{}, struct{}](ts.FsLib, mgr.Id)
	tasks := make([]*fttaskclnt.Task[struct{}], 0)
	for i := 0; i < ntasks; i++ {
		tasks = append(tasks, &fttaskclnt.Task[struct{}]{
			Id: int32(ids[i]),
			Data: struct{}{},
		})
	}

	db.DPrintf(db.TEST, "Submitting tasks %v", tasks)
	existing, err := clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))
	testServerContents(t, clnt,
		ids,
		[]int32{},
		[]int32{},
		[]int32{},
	)

	db.DPrintf(db.TEST, "Acquiring tasks")
	received, stopped, err := clnt.AcquireTasks(false)
	assert.Nil(t, err)
	assert.False(t, stopped)
	for i := 0; i < ntasks; i++ {
		found := false
		for _, id := range received {
			if id == int32(i) {
				found = true
			}
		}
		assert.True(t, found)
	}
	testServerContents(t, clnt,
		[]int32{},
		ids,
		[]int32{},
		[]int32{},
	)

	db.DPrintf(db.TEST, "Marking tasks as done")
	err = clnt.MoveTasksByStatus(proto.TaskStatus_WIP, proto.TaskStatus_DONE)
	assert.Nil(t, err)
	testServerContents(t, clnt,
		[]int32{},
		[]int32{},
		ids,
		[]int32{},
	)

	db.DPrintf(db.TEST, "Marking tasks as errored")
	err = clnt.MoveTasksByStatus(proto.TaskStatus_DONE, proto.TaskStatus_ERROR)
	assert.Nil(t, err)
	testServerContents(t, clnt,
		[]int32{},
		[]int32{},
		[]int32{},
		ids,
	)

	err = mgr.Stop()
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerMoveTasksById(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Making fttasks server")
	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test")
	assert.Nil(t, err)

	clnt := fttaskclnt.NewFtTaskClnt[struct{}, struct{}](ts.FsLib, mgr.Id)
	tasks := make([]*fttaskclnt.Task[struct{}], 0)
	for i := 0; i < 5; i++ {
		tasks = append(tasks, &fttaskclnt.Task[struct{}]{
			Id: int32(i),
			Data: struct{}{},
		})
	}

	db.DPrintf(db.TEST, "Submitting tasks %v", tasks)
	existing, err := clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))
	testServerContents(t, clnt,
		[]int32{0, 1, 2, 3, 4},
		[]int32{},
		[]int32{},
		[]int32{},
	)

	db.DPrintf(db.TEST, "Moving tasks to error")
	err = clnt.MoveTasks([]int32{0, 1}, proto.TaskStatus_ERROR)
	assert.Nil(t, err)
	testServerContents(t, clnt,
		[]int32{2, 3, 4},
		[]int32{},
		[]int32{},
		[]int32{0, 1},
	)

	db.DPrintf(db.TEST, "Moving tasks to wip")
	err = clnt.MoveTasks([]int32{1, 2}, proto.TaskStatus_WIP)
	assert.Nil(t, err)
	testServerContents(t, clnt,
		[]int32{3, 4},
		[]int32{1, 2},
		[]int32{},
		[]int32{0},
	)

	db.DPrintf(db.TEST, "Moving tasks to done")
	err = clnt.MoveTasks([]int32{3, 4}, proto.TaskStatus_DONE)
	assert.Nil(t, err)
	testServerContents(t, clnt,
		[]int32{},
		[]int32{1, 2},
		[]int32{3, 4},
		[]int32{0},
	)

	err = mgr.Stop()
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerData(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	ntasks := 5

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test")
	assert.Nil(t, err)

	clnt := fttaskclnt.NewFtTaskClnt[mr.Bin, string](ts.FsLib, mgr.Id)
	tasks := make([]*fttaskclnt.Task[mr.Bin], 0)
	for i := 0; i < ntasks; i++ {
		bin := make(mr.Bin, 1)
		bin[0].File = fmt.Sprintf("hello_%d", i)

		tasks = append(tasks, &fttaskclnt.Task[mr.Bin]{
			Id: int32(i),
			Data: bin,
		})
	}

	existing, err := clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	ids, stopped, err := clnt.AcquireTasks(false)
	assert.Nil(t, err)
	assert.False(t, stopped)
	assert.Equal(t, ntasks, len(ids))

	read, err := clnt.ReadTasks(ids)
	assert.Nil(t, err)
	assert.Equal(t, ntasks, len(read))
	for i := 0; i < ntasks; i++ {
		id := read[i].Id
		file := read[i].Data[0].File
		assert.Equal(t, fmt.Sprintf("hello_%d", id), file)
	}

	outputs := make([]string, len(ids))
	for i := 0; i < ntasks; i++ {
		outputs[i] = fmt.Sprintf("output_%d", i)
	}
	err = clnt.AddTaskOutputs(ids, outputs)
	assert.Nil(t, err)

	err = clnt.MoveTasksByStatus(proto.TaskStatus_WIP, proto.TaskStatus_DONE)
	assert.Nil(t, err)

	readOutputs, err := clnt.GetTaskOutputs(ids)
	assert.Nil(t, err)
	assert.Equal(t, ntasks, len(outputs))
	for i := 0; i < ntasks; i++ {
		assert.Equal(t, outputs[i], readOutputs[i])
	}

	err = mgr.Stop()
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerWait(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	ntasks := 5

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test")
	assert.Nil(t, err)

	clnt := fttaskclnt.NewFtTaskClnt[mr.Bin, string](ts.FsLib, mgr.Id)
	tasks := make([]*fttaskclnt.Task[mr.Bin], 0)
	for i := 0; i < ntasks; i++ {
		bin := make(mr.Bin, 1)
		bin[0].File = fmt.Sprintf("hello_%d", i)

		tasks = append(tasks, &fttaskclnt.Task[mr.Bin]{
			Id: int32(i),
			Data: bin,
		})
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		existing, err := clnt.SubmitTasks(tasks)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(existing))
	}()

	ids, stopped, err := clnt.AcquireTasks(true)
	assert.Nil(t, err)
	assert.False(t, stopped)
	assert.Equal(t, ntasks, len(ids))

	err = mgr.Stop()
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerErrors(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	ntasks := 5

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test")
	assert.Nil(t, err)

	clnt := fttaskclnt.NewFtTaskClnt[interface{}, interface{}](ts.FsLib, mgr.Id)
	tasks := make([]*fttaskclnt.Task[interface{}], 0)
	for i := 0; i < ntasks; i++ {
		tasks = append(tasks, &fttaskclnt.Task[interface{}]{
			Id: int32(i),
			Data: struct{}{},
		})
	}

	existing, err := clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	existing, err = clnt.SubmitTasks(tasks[:1])
	assert.Nil(t, err)
	assert.Equal(t, []int32{0}, existing)

	err = clnt.MoveTasks([]int32{5}, proto.TaskStatus_DONE)
	assert.NotNil(t, err)

	_, err = clnt.ReadTasks([]int32{6})
	assert.NotNil(t, err)

	_, err = clnt.GetTaskOutputs([]int32{0})
	assert.NotNil(t, err)

	_, err = clnt.GetTaskOutputs([]int32{6})
	assert.NotNil(t, err)

	err = mgr.Stop()
	assert.Nil(t, err)

	ts.Shutdown()
}