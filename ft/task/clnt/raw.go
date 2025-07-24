// Defines fttask client that sends just tasks as raw bytes
package clnt

import (
	"sync"
	"sync/atomic"

	protobuf "google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	fttask "sigmaos/ft/task"
	"sigmaos/ft/task/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/retry"
)

type RawFtTaskClnt struct {
	rpcclntc   *rpcclnt.ClntCache
	fsl        *fslib.FsLib
	serviceId  fttask.FtTaskSvcId
	fence      *sp.Tfence
	mu         sync.Mutex
	acquireId  *AcquireId
	acquireCnt atomic.Int64
}

func newRawFtTaskClnt(fsl *fslib.FsLib, serviceId fttask.FtTaskSvcId, a *AcquireId) *RawFtTaskClnt {
	tc := &RawFtTaskClnt{
		fsl:       fsl,
		rpcclntc:  rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
		serviceId: serviceId,
		fence:     nil,
		mu:        sync.Mutex{},
		acquireId: a,
	}
	return tc
}

func (tc *RawFtTaskClnt) fenceProto() *sp.TfenceProto {
	if tc.fence == nil {
		return nil
	}

	return tc.fence.FenceProto()
}

func (tc *RawFtTaskClnt) rpc(method string, arg protobuf.Message, res protobuf.Message) error {
	pn := tc.serviceId.ServicePath()
	return tc.rpcclntc.RPCRetryNotFound(pn, tc.serviceId.String(), method, arg, res)
}

func (tc *RawFtTaskClnt) SubmitTasks(tasks []*Task[[]byte]) error {
	var protoTasks []*proto.Task

	for _, task := range tasks {
		protoTasks = append(protoTasks, &proto.Task{
			Id:   task.Id,
			Data: task.Data,
		})
	}

	arg := proto.SubmitTasksReq{Tasks: protoTasks, Fence: tc.fenceProto()}
	res := proto.SubmitTasksRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.SubmitTasks", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "SubmitTasks: rpc err %v", err)
		}
		return err
	})

	return err
}

// EditTasks assumes only one client invokes it for a specific id
func (tc *RawFtTaskClnt) EditTasks(tasks []*Task[[]byte]) ([]TaskId, error) {
	var protoTasks []*proto.Task

	for _, task := range tasks {
		protoTasks = append(protoTasks, &proto.Task{
			Id:   task.Id,
			Data: task.Data,
		})
	}

	arg := proto.EditTasksReq{Tasks: protoTasks, Fence: tc.fenceProto()}
	res := proto.EditTasksRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.EditTasks", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "EditTasks: rpc err %v", err)
		}
		return err
	})
	return res.Unknown, err
}

func (tc *RawFtTaskClnt) GetTasksByStatus(taskStatus TaskStatus) ([]TaskId, error) {
	return tc.getTasksByStatusAcquirer(taskStatus, sp.NullFence())
}

func (tc *RawFtTaskClnt) getTasksByStatusAcquirer(taskStatus TaskStatus, aid *sp.Tfence) ([]TaskId, error) {
	arg := proto.GetTasksByStatusReq{
		Status:    taskStatus,
		Fence:     tc.fenceProto(),
		AcquireId: aid.FenceProto(),
	}
	res := proto.GetTasksByStatusRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.GetTasksByStatus", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "GetTasksByStatus: rpc err %v", err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return res.Ids, err
}

func (tc *RawFtTaskClnt) ReadTasks(ids []TaskId) ([]Task[[]byte], error) {
	arg := proto.ReadTasksReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.ReadTasksRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.ReadTasks", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "ReadTasks: rpc err %v", err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	var tasks []Task[[]byte]
	for _, protoTask := range res.Tasks {
		tasks = append(tasks, Task[[]byte]{Id: protoTask.Id, Data: protoTask.Data})
	}

	return tasks, nil
}

// MoveTasks assumes only one client invokes it for a specific id
func (tc *RawFtTaskClnt) MoveTasks(ids []TaskId, to TaskStatus) error {
	arg := proto.MoveTasksReq{Ids: ids, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.MoveTasks", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "MoveTasks: rpc err %v", err)
		}
		return err
	})
	return err
}

// MoveTasksByStatus assumes only one client invokes it
func (tc *RawFtTaskClnt) MoveTasksByStatus(from, to TaskStatus) (int32, error) {
	arg := proto.MoveTasksByStatusReq{From: from, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksByStatusRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.MoveTasksByStatus", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "MoveTasksByStatus: rpc err %v", err)
		}
		return err
	})
	return res.NumMoved, err
}

func (tc *RawFtTaskClnt) GetTaskOutputs(ids []TaskId) ([][]byte, error) {
	arg := proto.GetTaskOutputsReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.GetTaskOutputsRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.GetTaskOutputs", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "GetTaskOutputs: rpc err %v", err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return res.Outputs, nil
}

// AddTaskOutput assumes only one client invokes it for a specific id
func (tc *RawFtTaskClnt) AddTaskOutputs(ids []TaskId, outputs [][]byte, markDone bool) error {
	arg := proto.AddTaskOutputsReq{Ids: ids, Outputs: outputs, MarkDone: markDone, Fence: tc.fenceProto()}
	res := proto.AddTaskOutputsRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.AddTaskOutputs", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "AddTasksOutputs: rpc err %v", err)
		}
		return err
	})
	return err
}

func (tc *RawFtTaskClnt) AcquireTasks(wait bool) ([]TaskId, bool, error) {
	f := sp.NoFence()
	if tc.acquireId != nil {
		cnt := tc.acquireCnt.Add(1)
		f = *tc.acquireId
		f.Seqno = sp.Tseqno(cnt)
		db.DPrintf(db.FTTASKCLNT, "f %v %v", f, cnt)
	}

	arg := proto.AcquireTasksReq{
		Wait:      wait,
		Fence:     tc.fenceProto(),
		AcquireId: f.FenceProto(),
	}
	res := proto.AcquireTasksRep{}

	err, ok := retry.RetryAtMostOnce(func() error {
		err := tc.rpc("TaskSrv.AcquireTasks", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "AcquireTasks: rpc err %v", err)
		}
		return err
	})
	if !ok {
		return nil, false, serr.NewErr(serr.TErrUnreachable, tc.serviceId.ServicePath())
	}
	if serr.IsErrorIO(err) {
		tasks, err := tc.getTasksByStatusAcquirer(WIP, &f)
		if err != nil {
			return nil, false, err
		}
		return tasks, false, err // XXX stopped?
	}
	return res.Ids, res.Stopped, err
}

func (tc *RawFtTaskClnt) Stats() (*proto.TaskStats, error) {
	arg := proto.GetTaskStatsReq{}
	res := proto.GetTaskStatsRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.GetTaskStats", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "Stats: rpc err %v", err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return res.Stats, err
}

func (tc *RawFtTaskClnt) GetNTasks(status TaskStatus) (int32, error) {
	stats, err := tc.Stats()
	if err != nil {
		return 0, err
	}

	switch status {
	case TODO:
		return stats.NumTodo, nil
	case WIP:
		return stats.NumWip, nil
	case DONE:
		return stats.NumDone, nil
	case ERROR:
		return stats.NumError, nil
	default:
		return 0, serr.NewErr(serr.TErrInval, "invalid task status")
	}
}

func (tc *RawFtTaskClnt) SubmittedLastTask() error {
	arg := proto.SubmittedLastTaskReq{Fence: tc.fenceProto()}
	res := proto.SubmittedLastTaskRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.SubmittedLastTask", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "SubmittedLastTask: rpc err %v", err)
		}
		return err
	})
	return err
}

func (tc *RawFtTaskClnt) SetFence(fence *sp.Tfence) {
	tc.fence = fence
}

func (tc *RawFtTaskClnt) GetFence() *sp.Tfence {
	return tc.fence
}

func (tc *RawFtTaskClnt) Fence(fence *sp.Tfence) error {
	tc.SetFence(fence)

	arg := proto.FenceReq{Fence: tc.fenceProto()}
	res := proto.FenceRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.Fence", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "Fence: rpc err %v", err)
		}
		return err
	})
	return err
}

// ClearEtcd assumes only one client executes it
func (tc *RawFtTaskClnt) ClearEtcd() error {
	arg := proto.ClearEtcdReq{}
	res := proto.ClearEtcdRep{}

	err, _ := retry.RetryAtLeastOnce(func() error {
		err := tc.rpc("TaskSrv.ClearEtcd", &arg, &res)
		if err != nil {
			db.DPrintf(db.FTTASKCLNT, "ClearEtcd: rpc err %v", err)
		}
		return err
	})
	return err
}

func (tc *RawFtTaskClnt) AsRawClnt() FtTaskClnt[[]byte, []byte] {
	return tc
}

func (tc *RawFtTaskClnt) ServiceId() fttask.FtTaskSvcId {
	return tc.serviceId
}
