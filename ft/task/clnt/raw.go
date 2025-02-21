package clnt

import (
	"path/filepath"
	"sigmaos/ft/task/proto"
	fttask_srv "sigmaos/ft/task/srv"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type RawFtTaskClnt struct {
	rpc         *rpcclnt.ClntCache
	fsl         *fslib.FsLib
	serverPath  string
	serverId    fttask_srv.FtTaskSrvId
	fence       *sp.Tfence
}

func newRawFtTaskClnt(fsl *fslib.FsLib, serverId fttask_srv.FtTaskSrvId) *RawFtTaskClnt {
	tc := &RawFtTaskClnt{
		fsl:         fsl,
		rpc:         rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
		serverPath: filepath.Join(sp.NAMED, "fttask", string(serverId)),
		serverId:   serverId,
	}
	return tc
}

func (tc *RawFtTaskClnt) fenceProto() *sp.TfenceProto {
	if tc.fence == nil {
		return nil
	}

	return tc.fence.FenceProto()
}

func (tc *RawFtTaskClnt) SubmitTasks(tasks []*Task[[]byte]) ([]TaskId, error) {
	var protoTasks []*proto.Task

	for _, task := range tasks {
		protoTasks = append(protoTasks, &proto.Task{
			Id:   task.Id,
			Data: task.Data,
		})
	}

	arg := proto.SubmitTasksReq{Tasks: protoTasks, Fence: tc.fenceProto()}
	res := proto.SubmitTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.SubmitTasks", &arg, &res)
	return res.Existing, err
}

func (tc *RawFtTaskClnt) GetTasksByStatus(taskStatus TaskStatus) ([]TaskId, error) {
	arg := proto.GetTasksByStatusReq{Status: taskStatus, Fence: tc.fenceProto()}
	res := proto.GetTasksByStatusRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTasksByStatus", &arg, &res)
	return res.Ids, err
}

func (tc *RawFtTaskClnt) ReadTasks(ids []TaskId) ([]Task[[]byte], error) {
	arg := proto.ReadTasksReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.ReadTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.ReadTasks", &arg, &res)
	if err != nil {
		return nil, err
	}

	var tasks []Task[[]byte]
	for _, protoTask := range res.Tasks {
		tasks = append(tasks, Task[[]byte]{Id: protoTask.Id, Data: protoTask.Data})
	}

	return tasks, nil
}

func (tc *RawFtTaskClnt) MoveTasks(ids []TaskId, to TaskStatus) error {
	arg := proto.MoveTasksReq{Ids: ids, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.MoveTasks", &arg, &res)
	return err
}

func (tc *RawFtTaskClnt) MoveTasksByStatus(from, to TaskStatus) (int32, error) {
	arg := proto.MoveTasksByStatusReq{From: from, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksByStatusRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.MoveTasksByStatus", &arg, &res)
	return res.NumMoved, err
}

func (tc *RawFtTaskClnt) GetTaskOutputs(ids []TaskId) ([][]byte, error) {
	arg := proto.GetTaskOutputsReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.GetTaskOutputsRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTaskOutputs", &arg, &res)
	if err != nil {
		return nil, err
	}

	return res.Outputs, nil
}

func (tc *RawFtTaskClnt) AddTaskOutputs(ids []TaskId, outputs [][]byte) error {
	arg := proto.AddTaskOutputsReq{Ids: ids, Outputs: outputs, Fence: tc.fenceProto()}
	res := proto.AddTaskOutputsRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.AddTaskOutputs", &arg, &res)
	return err
}

func (tc *RawFtTaskClnt) AcquireTasks(wait bool) ([]TaskId, bool, error) {
	arg := proto.AcquireTasksReq{Wait: wait, Fence: tc.fenceProto()}
	res := proto.AcquireTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.AcquireTasks", &arg, &res)
	return res.Ids, res.Stopped, err
}

func (tc *RawFtTaskClnt) Stats() (*proto.TaskStats, error) {
	arg := proto.GetTaskStatsReq{}
	res := proto.GetTaskStatsRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTaskStats", &arg, &res)
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

func (tc *RawFtTaskClnt) SubmitStop() error {
	arg := proto.SubmitStopReq{Fence: tc.fenceProto()}
	res := proto.SubmitStopRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.SubmitStop", &arg, &res)
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

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.Fence", &arg, &res)
	return err
}

func (tc *RawFtTaskClnt) Raw() FtTaskClnt[[]byte, []byte] {
	return tc
}

func (tc *RawFtTaskClnt) ServerId() fttask_srv.FtTaskSrvId {
	return tc.serverId
}