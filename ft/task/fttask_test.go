package task_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/ft/procgroupmgr"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/ft/task/proto"
	fttask_srv "sigmaos/ft/task/srv"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	"sigmaos/util/retry"
)

const (
	TASKS = "name/tasks"
)

func TestCompile(t *testing.T) {
}

func testServerContents[Data any, Output any](t *testing.T, clnt fttask_clnt.FtTaskClnt[Data, Output], todo []int32, wip []int32, done []int32, err []int32) {
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
	assert.Equal(t, int32(len(wip)), stats.NumWip)
	assert.Equal(t, int32(len(done)), stats.NumDone)
	assert.Equal(t, int32(len(err)), stats.NumError)
}

type Tstate[Data any, Output any] struct {
	*test.Tstate
	mgr  *fttask_srv.FtTaskSrvMgr
	clnt fttask_clnt.FtTaskClnt[Data, Output]
}

func newTstate[Data any, Output any](t *testing.T) (*Tstate[Data, Output], error) {
	ts := &Tstate[Data, Output]{}
	ts0, err := test.NewTstateAll(t)
	if err != nil {
		return nil, err
	}
	ts.Tstate = ts0
	mgr, err := fttask_srv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", false)
	if err != nil {
		return nil, err
	}
	ts.mgr = mgr
	ts.clnt = fttask_clnt.NewFtTaskClnt[Data, Output](ts.FsLib, mgr.Id)
	return ts, nil
}

func (ts *Tstate[Data, Output]) shutdown() []*procgroupmgr.ProcStatus {
	stats, err := ts.mgr.Stop(true)
	assert.Nil(ts.T, err)
	ts.Shutdown()
	return stats
}

func TestServerPerf(t *testing.T) {
	nTasks := 1000

	ts, err := newTstate[mr.Bin, string](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	start := time.Now()
	for i := 0; i < nTasks; i++ {
		tasks := []*fttask_clnt.Task[mr.Bin]{
			{
				Id: int32(i),
				Data: mr.Bin{
					{
						File: "hello",
					},
				},
			},
		}
		existing, err := ts.clnt.SubmitTasks(tasks)
		assert.Nil(t, err)
		assert.Empty(t, existing)
	}
	db.DPrintf(db.ALWAYS, "Submitting tasks took %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	ids, _, err := ts.clnt.AcquireTasks(false)
	db.DPrintf(db.ALWAYS, "Acquired tasks in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(ids))

	start = time.Now()
	for _, id := range ids {
		b, err := ts.clnt.ReadTasks([]fttask_clnt.TaskId{id})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(b))
		assert.Equal(t, "hello", b[0].Data[0].File)
	}
	db.DPrintf(db.ALWAYS, "Read tasks in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	for _, id := range ids {
		err = ts.clnt.AddTaskOutputs([]fttask_clnt.TaskId{id}, []string{"bye"}, true)
		assert.Nil(t, err)
	}
	db.DPrintf(db.ALWAYS, "Marked all tasks done in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	for _, id := range ids {
		output, err := ts.clnt.GetTaskOutputs([]fttask_clnt.TaskId{id})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(output))
		assert.Equal(t, "bye", output[0])
	}
	db.DPrintf(db.ALWAYS, "Read all outputs in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	ts.shutdown()
}

func TestServerBatchedPerf(t *testing.T) {
	nTasks := 1000

	ts, err := newTstate[mr.Bin, string](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	tasks := make([]*fttask_clnt.Task[mr.Bin], nTasks)
	for i := 0; i < nTasks; i++ {
		tasks[i] = &fttask_clnt.Task[mr.Bin]{
			Id: int32(i),
			Data: mr.Bin{
				{
					File: "hello",
				},
			},
		}
	}

	start := time.Now()
	existing, err := ts.clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Empty(t, existing)
	db.DPrintf(db.ALWAYS, "Submitting tasks took %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	ids, _, err := ts.clnt.AcquireTasks(false)
	db.DPrintf(db.ALWAYS, "Acquired tasks in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(ids))

	start = time.Now()
	b, err := ts.clnt.ReadTasks(ids)
	db.DPrintf(db.ALWAYS, "Read tasks in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(b))
	for _, task := range b {
		assert.Equal(t, "hello", task.Data[0].File)
	}

	start = time.Now()
	outputs := make([]string, nTasks)
	for i := 0; i < nTasks; i++ {
		outputs[i] = "bye"
	}
	err = ts.clnt.AddTaskOutputs(ids, outputs, true)
	assert.Nil(t, err)
	db.DPrintf(db.ALWAYS, "Marked all tasks done in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	output, err := ts.clnt.GetTaskOutputs(ids)
	db.DPrintf(db.ALWAYS, "Read all outputs in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(output))
	for _, out := range output {
		assert.Equal(t, "bye", out)
	}
	ts.shutdown()
}

func TestServerMoveTasksByStatus(t *testing.T) {
	ts, err := newTstate[struct{}, struct{}](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	ntasks := 5
	ids := make([]int32, ntasks)
	for i := 0; i < ntasks; i++ {
		ids[i] = int32(i)
	}

	db.DPrintf(db.TEST, "Making fttasks server")
	tasks := make([]*fttask_clnt.Task[struct{}], 0)
	for i := 0; i < ntasks; i++ {
		tasks = append(tasks, &fttask_clnt.Task[struct{}]{
			Id:   int32(ids[i]),
			Data: struct{}{},
		})
	}

	db.DPrintf(db.TEST, "Submitting tasks %v", tasks)
	existing, err := ts.clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))
	testServerContents(t, ts.clnt,
		ids,
		[]int32{},
		[]int32{},
		[]int32{},
	)

	db.DPrintf(db.TEST, "Acquiring tasks")
	received, stopped, err := ts.clnt.AcquireTasks(false)
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
	testServerContents(t, ts.clnt,
		[]int32{},
		ids,
		[]int32{},
		[]int32{},
	)

	db.DPrintf(db.TEST, "Marking tasks as done")
	n, err := ts.clnt.MoveTasksByStatus(proto.TaskStatus_WIP, proto.TaskStatus_DONE)
	assert.Nil(t, err)
	assert.Equal(t, ntasks, int(n))
	testServerContents(t, ts.clnt,
		[]int32{},
		[]int32{},
		ids,
		[]int32{},
	)

	db.DPrintf(db.TEST, "Marking tasks as errored")
	n, err = ts.clnt.MoveTasksByStatus(proto.TaskStatus_DONE, proto.TaskStatus_ERROR)
	assert.Nil(t, err)
	assert.Equal(t, ntasks, int(n))
	testServerContents(t, ts.clnt,
		[]int32{},
		[]int32{},
		[]int32{},
		ids,
	)

	ts.shutdown()
}

func TestServerMoveTasksById(t *testing.T) {
	ts, err := newTstate[struct{}, struct{}](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	tasks := make([]*fttask_clnt.Task[struct{}], 0)
	for i := 0; i < 5; i++ {
		tasks = append(tasks, &fttask_clnt.Task[struct{}]{
			Id:   int32(i),
			Data: struct{}{},
		})
	}

	db.DPrintf(db.TEST, "Submitting tasks %v", tasks)
	existing, err := ts.clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))
	testServerContents(t, ts.clnt,
		[]int32{0, 1, 2, 3, 4},
		[]int32{},
		[]int32{},
		[]int32{},
	)

	db.DPrintf(db.TEST, "Moving tasks to error")
	err = ts.clnt.MoveTasks([]int32{0, 1}, proto.TaskStatus_ERROR)
	assert.Nil(t, err)
	testServerContents(t, ts.clnt,
		[]int32{2, 3, 4},
		[]int32{},
		[]int32{},
		[]int32{0, 1},
	)

	db.DPrintf(db.TEST, "Moving tasks to wip")
	err = ts.clnt.MoveTasks([]int32{1, 2}, proto.TaskStatus_WIP)
	assert.Nil(t, err)
	testServerContents(t, ts.clnt,
		[]int32{3, 4},
		[]int32{1, 2},
		[]int32{},
		[]int32{0},
	)

	db.DPrintf(db.TEST, "Moving tasks to done")
	err = ts.clnt.MoveTasks([]int32{3, 4}, proto.TaskStatus_DONE)
	assert.Nil(t, err)
	testServerContents(t, ts.clnt,
		[]int32{},
		[]int32{1, 2},
		[]int32{3, 4},
		[]int32{0},
	)

	ts.shutdown()
}

func TestServerWait(t *testing.T) {
	ts, err := newTstate[mr.Bin, string](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	ntasks := 5
	tasks := make([]*fttask_clnt.Task[mr.Bin], 0)
	for i := 0; i < ntasks; i++ {
		bin := make(mr.Bin, 1)
		bin[0].File = fmt.Sprintf("hello_%d", i)

		tasks = append(tasks, &fttask_clnt.Task[mr.Bin]{
			Id:   int32(i),
			Data: bin,
		})
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		existing, err := ts.clnt.SubmitTasks(tasks)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(existing))
	}()

	ids, stopped, err := ts.clnt.AcquireTasks(true)
	assert.Nil(t, err)
	assert.False(t, stopped)
	assert.Equal(t, ntasks, len(ids))

	ts.shutdown()
}

// Check that server returns an error for a few incorrect calls
func TestServerErrors(t *testing.T) {
	ts, err := newTstate[interface{}, interface{}](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	ntasks := 5
	tasks := make([]*fttask_clnt.Task[interface{}], 0)
	for i := 0; i < ntasks; i++ {
		tasks = append(tasks, &fttask_clnt.Task[interface{}]{
			Id:   int32(i),
			Data: struct{}{},
		})
	}

	existing, err := ts.clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	existing, err = ts.clnt.SubmitTasks(tasks[:1])
	assert.Nil(t, err)
	assert.Equal(t, []int32{0}, existing)

	err = ts.clnt.MoveTasks([]int32{5}, proto.TaskStatus_DONE)
	assert.NotNil(t, err)

	_, err = ts.clnt.ReadTasks([]int32{6})
	assert.NotNil(t, err)

	_, err = ts.clnt.GetTaskOutputs([]int32{0})
	assert.NotNil(t, err)

	_, err = ts.clnt.GetTaskOutputs([]int32{6})
	assert.NotNil(t, err)

	ts.shutdown()
}

func TestServerStop(t *testing.T) {
	ts, err := newTstate[interface{}, interface{}](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	existing, err := ts.clnt.SubmitTasks([]*fttask_clnt.Task[interface{}]{
		{
			Id:   int32(0),
			Data: struct{}{},
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	_, stopped, err := ts.clnt.AcquireTasks(true)
	assert.Nil(t, err)
	assert.False(t, stopped)

	n, err := ts.clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.DONE)
	assert.Nil(t, err)
	assert.Equal(t, 1, int(n))

	err = ts.clnt.SubmittedLastTask()
	assert.Nil(t, err)

	_, stopped, err = ts.clnt.AcquireTasks(true)
	assert.Nil(t, err)
	assert.True(t, stopped)
	assert.Equal(t, 0, len(existing))

	ts.shutdown()
}

func TestGetTasksClose(t *testing.T) {
	ts, err := newTstate[interface{}, interface{}](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	chTasks := make(chan []fttask_clnt.TaskId)
	go fttask_clnt.GetTasks[interface{}, interface{}](ts.clnt, chTasks)

	_, err = ts.clnt.SubmitTasks([]*fttask_clnt.Task[interface{}]{
		{
			Id:   int32(0),
			Data: struct{}{},
		},
	})
	assert.Nil(t, err)

	err = ts.clnt.SubmittedLastTask()
	assert.Nil(ts.T, err)

	n := 0
	for range chTasks {
		if n == 0 {
			// but move wip task to do do (simulating a failed task), which
			// will show up chTasks again
			err = ts.clnt.MoveTasks([]fttask_clnt.TaskId{0}, fttask_clnt.TODO)
			assert.Nil(t, err)
		} else {
			err = ts.clnt.MoveTasks([]fttask_clnt.TaskId{0}, fttask_clnt.DONE)
			assert.Nil(t, err)
		}
		n += 1
	}

	assert.Equal(t, 2, n)

	ts.shutdown()
}

func TestErrorTasks(t *testing.T) {
	ts, err := newTstate[interface{}, string](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	chTasks := make(chan []fttask_clnt.TaskId)
	go fttask_clnt.GetTasks[interface{}, string](ts.clnt, chTasks)

	_, err = ts.clnt.SubmitTasks([]*fttask_clnt.Task[interface{}]{
		{
			Id:   int32(0),
			Data: struct{}{},
		},
	})
	assert.Nil(t, err)

	err = ts.clnt.SubmittedLastTask()
	assert.Nil(ts.T, err)

	for range chTasks {
		err = ts.clnt.MoveTasks([]fttask_clnt.TaskId{0}, fttask_clnt.ERROR)
		assert.Nil(t, err)
		o := []string{"error"}
		err = ts.clnt.AddTaskOutputs([]fttask_clnt.TaskId{0}, o, false)
		assert.Nil(t, err)
	}

	ids, err := ts.clnt.GetTasksByStatus(fttask_clnt.ERROR)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ids))

	outs, err := ts.clnt.GetTaskOutputs(ids)
	for _, o := range outs {
		assert.Equal(t, "error", o)
	}

	ts.shutdown()
}

func TestServerFence(t *testing.T) {
	ts, err := newTstate[interface{}, interface{}](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	n, err := ts.clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.Nil(t, err)
	assert.Equal(t, 0, int(n))

	fence := &sigmap.Tfence{PathName: "test", Epoch: 1, Seqno: 0}
	ts.clnt.SetFence(fence)
	n, err = ts.clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.Nil(t, err)
	assert.Equal(t, 0, int(n))

	ts.clnt.Fence(fence)
	n, err = ts.clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.Nil(t, err)
	assert.Equal(t, 0, int(n))

	ts.clnt.SetFence(sigmap.NullFence())
	_, err = ts.clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.NotNil(t, err)

	ts.shutdown()
}

func runTestServerData(t *testing.T, em *crash.TeventMap) []*procgroupmgr.ProcStatus {
	ts, err := newTstate[mr.Bin, string](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return nil
	}

	ntasks := 5

	err = crash.SetSigmaFail(em)
	assert.Nil(t, err)

	tasks := make([]*fttask_clnt.Task[mr.Bin], 0)
	for i := 0; i < ntasks; i++ {
		bin := make(mr.Bin, 1)
		bin[0].File = fmt.Sprintf("hello_%d", i)

		tasks = append(tasks, &fttask_clnt.Task[mr.Bin]{
			Id:   int32(i),
			Data: bin,
		})
	}

	existing, err := ts.clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	ids, stopped, err := ts.clnt.AcquireTasks(false)
	assert.Nil(t, err)
	assert.False(t, stopped)
	assert.Equal(t, ntasks, len(ids))

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	read, err := ts.clnt.ReadTasks(ids)
	assert.Nil(t, err)
	assert.Equal(t, ntasks, len(read))
	for i := 0; i < ntasks; i++ {
		id := read[i].Id
		file := read[i].Data[0].File
		assert.Equal(t, fmt.Sprintf("hello_%d", id), file)
	}

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	outputs := make([]string, len(ids))
	for i := 0; i < ntasks; i++ {
		outputs[i] = fmt.Sprintf("output_%d", i)
	}
	err = ts.clnt.AddTaskOutputs(ids, outputs, false)
	assert.Nil(t, err)

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	n, err := ts.clnt.MoveTasksByStatus(proto.TaskStatus_WIP, proto.TaskStatus_DONE)
	assert.Equal(t, ntasks, int(n))
	assert.Nil(t, err)

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	var readOutputs []string
	err, ok := retry.RetryAtLeastOnce(func() error {
		readOutputs, err = ts.clnt.GetTaskOutputs(ids)
		if err == nil {
			assert.Equal(t, ntasks, len(outputs))
		}
		return err
	})
	assert.Nil(t, err)
	assert.True(t, ok)

	for i := 0; i < ntasks; i++ {
		assert.Equal(t, outputs[i], readOutputs[i])
	}

	return ts.shutdown()
}

func TestClntPartition(t *testing.T) {
	ts, err := newTstate[mr.Bin, string](t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	assert.Nil(t, err)
	clnt := fttask_clnt.NewFtTaskClnt[mr.Bin, string](fsl, ts.mgr.Id)

	n, err := clnt.GetNTasks(fttask_clnt.TODO)
	assert.Nil(t, err)

	crash.PartitionPath(fsl, filepath.Join(ts.clnt.ServerId().ServerPath(), clnt.CurrInstance()))
	_, _, err = clnt.AcquireTasks(false)
	assert.NotNil(t, err)

	ids, _, err := ts.clnt.AcquireTasks(false)
	assert.Nil(t, err)

	assert.Equal(t, int(n), len(ids))

	ts.shutdown()
}

func TestServerData(t *testing.T) {
	runTestServerData(t, nil)
}

func TestServerCrash(t *testing.T) {
	succ := false
	e0 := crash.NewEventStart(crash.FTTASKS_CRASH, 50, 250, 0.33)
	for i := 0; i < 10; i++ {
		stats := runTestServerData(t, crash.NewTeventMapOne(e0))
		db.DPrintf(db.ALWAYS, "restarted %d times", stats[0].Nstart)
		if stats[0].Nstart > 1 {
			succ = true
			break
		}
	}
	assert.True(t, succ)
}

func TestServerPartition(t *testing.T) {
}
