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
	fttasksrv "sigmaos/ft/task/srv"
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

func TestServerPerf(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	nTasks := 1000

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[mr.Bin, string](ts.FsLib, mgr.Id)

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
		existing, err := clnt.SubmitTasks(tasks)
		assert.Nil(t, err)
		assert.Empty(t, existing)
	}
	db.DPrintf(db.ALWAYS, "Submitting tasks took %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	ids, _, err := clnt.AcquireTasks(false)
	db.DPrintf(db.ALWAYS, "Acquired tasks in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(ids))

	start = time.Now()
	for _, id := range ids {
		b, err := clnt.ReadTasks([]fttask_clnt.TaskId{id})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(b))
		assert.Equal(t, "hello", b[0].Data[0].File)
	}
	db.DPrintf(db.ALWAYS, "Read tasks in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	for _, id := range ids {
		err = clnt.AddTaskOutputs([]fttask_clnt.TaskId{id}, []string{"bye"}, true)
		assert.Nil(t, err)
	}
	db.DPrintf(db.ALWAYS, "Marked all tasks done in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	for _, id := range ids {
		output, err := clnt.GetTaskOutputs([]fttask_clnt.TaskId{id})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(output))
		assert.Equal(t, "bye", output[0])
	}
	db.DPrintf(db.ALWAYS, "Read all outputs in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerBatchedPerf(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	nTasks := 1000

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[mr.Bin, string](ts.FsLib, mgr.Id)

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
	existing, err := clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Empty(t, existing)
	db.DPrintf(db.ALWAYS, "Submitting tasks took %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	ids, _, err := clnt.AcquireTasks(false)
	db.DPrintf(db.ALWAYS, "Acquired tasks in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(ids))

	start = time.Now()
	b, err := clnt.ReadTasks(ids)
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
	err = clnt.AddTaskOutputs(ids, outputs, true)
	assert.Nil(t, err)
	db.DPrintf(db.ALWAYS, "Marked all tasks done in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))

	start = time.Now()
	output, err := clnt.GetTaskOutputs(ids)
	db.DPrintf(db.ALWAYS, "Read all outputs in %v (%v per task)", time.Since(start), time.Since(start)/time.Duration(nTasks))
	assert.Nil(t, err)
	assert.Equal(t, nTasks, len(output))
	for _, out := range output {
		assert.Equal(t, "bye", out)
	}

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
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
	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[struct{}, struct{}](ts.FsLib, mgr.Id)
	tasks := make([]*fttask_clnt.Task[struct{}], 0)
	for i := 0; i < ntasks; i++ {
		tasks = append(tasks, &fttask_clnt.Task[struct{}]{
			Id:   int32(ids[i]),
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
	n, err := clnt.MoveTasksByStatus(proto.TaskStatus_WIP, proto.TaskStatus_DONE)
	assert.Nil(t, err)
	assert.Equal(t, ntasks, int(n))
	testServerContents(t, clnt,
		[]int32{},
		[]int32{},
		ids,
		[]int32{},
	)

	db.DPrintf(db.TEST, "Marking tasks as errored")
	n, err = clnt.MoveTasksByStatus(proto.TaskStatus_DONE, proto.TaskStatus_ERROR)
	assert.Nil(t, err)
	assert.Equal(t, ntasks, int(n))
	testServerContents(t, clnt,
		[]int32{},
		[]int32{},
		[]int32{},
		ids,
	)

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerMoveTasksById(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Making fttasks server")
	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[struct{}, struct{}](ts.FsLib, mgr.Id)
	tasks := make([]*fttask_clnt.Task[struct{}], 0)
	for i := 0; i < 5; i++ {
		tasks = append(tasks, &fttask_clnt.Task[struct{}]{
			Id:   int32(i),
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

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerWait(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	ntasks := 5

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[mr.Bin, string](ts.FsLib, mgr.Id)
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
		existing, err := clnt.SubmitTasks(tasks)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(existing))
	}()

	ids, stopped, err := clnt.AcquireTasks(true)
	assert.Nil(t, err)
	assert.False(t, stopped)
	assert.Equal(t, ntasks, len(ids))

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerErrors(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	ntasks := 5

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[interface{}, interface{}](ts.FsLib, mgr.Id)
	tasks := make([]*fttask_clnt.Task[interface{}], 0)
	for i := 0; i < ntasks; i++ {
		tasks = append(tasks, &fttask_clnt.Task[interface{}]{
			Id:   int32(i),
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

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerStop(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[interface{}, interface{}](ts.FsLib, mgr.Id)
	existing, err := clnt.SubmitTasks([]*fttask_clnt.Task[interface{}]{
		{
			Id:   int32(0),
			Data: struct{}{},
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	_, stopped, err := clnt.AcquireTasks(true)
	assert.Nil(t, err)
	assert.False(t, stopped)

	existing, err = clnt.SubmitTasks([]*fttask_clnt.Task[interface{}]{
		{
			Id:   int32(1),
			Data: struct{}{},
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	n, err := clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.Nil(t, err)
	assert.Equal(t, 1, int(n))
	err = clnt.SubmitStop()
	assert.Nil(t, err)

	_, stopped, err = clnt.AcquireTasks(true)
	assert.Nil(t, err)
	assert.True(t, stopped)
	assert.Equal(t, 0, len(existing))

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestServerFence(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[interface{}, interface{}](ts.FsLib, mgr.Id)
	n, err := clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.Nil(t, err)
	assert.Equal(t, 0, int(n))

	fence := &sigmap.Tfence{PathName: "test", Epoch: 1, Seqno: 0}
	clnt.SetFence(fence)
	n, err = clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.Nil(t, err)
	assert.Equal(t, 0, int(n))

	clnt.Fence(fence)
	n, err = clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.Nil(t, err)
	assert.Equal(t, 0, int(n))

	clnt.SetFence(sigmap.NullFence())
	_, err = clnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO)
	assert.NotNil(t, err)

	_, err = mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()
}

func runTestServerData(t *testing.T, em *crash.TeventMap) []*procgroupmgr.ProcStatus {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return nil
	}

	ntasks := 5

	err = crash.SetSigmaFail(em)
	assert.Nil(t, err)

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", true)
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[mr.Bin, string](ts.FsLib, mgr.Id)
	tasks := make([]*fttask_clnt.Task[mr.Bin], 0)
	for i := 0; i < ntasks; i++ {
		bin := make(mr.Bin, 1)
		bin[0].File = fmt.Sprintf("hello_%d", i)

		tasks = append(tasks, &fttask_clnt.Task[mr.Bin]{
			Id:   int32(i),
			Data: bin,
		})
	}

	existing, err := clnt.SubmitTasks(tasks)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(existing))

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	ids, stopped, err := clnt.AcquireTasks(false)
	assert.Nil(t, err)
	assert.False(t, stopped)
	assert.Equal(t, ntasks, len(ids))

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	read, err := clnt.ReadTasks(ids)
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
	err = clnt.AddTaskOutputs(ids, outputs, false)
	assert.Nil(t, err)

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	n, err := clnt.MoveTasksByStatus(proto.TaskStatus_WIP, proto.TaskStatus_DONE)
	assert.Equal(t, ntasks, int(n))
	assert.Nil(t, err)

	if em != nil {
		time.Sleep(30 * time.Millisecond)
	}

	var readOutputs []string
	err, ok := retry.RetryAtLeastOnce(func() error {
		readOutputs, err = clnt.GetTaskOutputs(ids)
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

	stats, err := mgr.Stop(true)
	assert.Nil(t, err)

	ts.Shutdown()

	return stats
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
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err, "Error New Tstate: %v", err)

	mgr, err := fttasksrv.NewFtTaskSrvMgr(ts.SigmaClnt, "test", false)
	assert.Nil(t, err)

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	assert.Nil(t, err)

	clnt := fttask_clnt.NewFtTaskClnt[mr.Bin, string](fsl, mgr.Id)

	n, err := clnt.GetNTasks(fttask_clnt.TODO)
	assert.Nil(t, err)

	crash.PartitionPath(fsl, filepath.Join(clnt.ServerId().ServerPath(), clnt.CurrInstance()))
	_, _, err = clnt.AcquireTasks(false)
	assert.NotNil(t, err)

	clnt = fttask_clnt.NewFtTaskClnt[mr.Bin, string](ts.FsLib, mgr.Id)

	ids, _, err := clnt.AcquireTasks(false)
	assert.Nil(t, err)

	assert.Equal(t, int(n), len(ids))

	stats, err := mgr.Stop(true)
	assert.Nil(t, err)

	assert.Equal(t, 1, stats[0].Nstart)

	err = ts.Shutdown()
	assert.Nil(t, err)
}
