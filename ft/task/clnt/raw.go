package clnt

import (
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/ft/task"
	"sigmaos/ft/task/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"time"

	protobuf "google.golang.org/protobuf/proto"
)

const (
	MAX_RETRIES = 3
	TIME_BETWEEN_RETRIES = 1 * time.Second
)

type RawFtTaskClnt struct {
	rpc         *rpcclnt.ClntCache
	fsl         *fslib.FsLib
	serverPath  string
	serverId    task.FtTaskSrvId
	fence       *sp.Tfence
}

func newRawFtTaskClnt(fsl *fslib.FsLib, serverId task.FtTaskSrvId) *RawFtTaskClnt {
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

func (tc *RawFtTaskClnt) RPC(method string, arg protobuf.Message, res protobuf.Message) error {
	var e error
	for i := 0; i < MAX_RETRIES; i++ {
		err := tc.rpc.RPC(tc.serverPath, method, arg, res)
		if err == nil {
			return nil
		}

		e = err
		if serr.IsErrorUnavailable(err) && err.(*serr.Err).Obj == tc.serverPath {
			db.DPrintf(db.FTTASKS, "RPC failed with err %v, retrying in %v", err, TIME_BETWEEN_RETRIES)
			time.Sleep(TIME_BETWEEN_RETRIES)
			continue
		}

		break
	}
	return e
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

	err := tc.RPC("TaskSrv.SubmitTasks", &arg, &res)
	return res.Existing, err
}

func (tc *RawFtTaskClnt) GetTasksByStatus(taskStatus TaskStatus) ([]TaskId, error) {
	arg := proto.GetTasksByStatusReq{Status: taskStatus, Fence: tc.fenceProto()}
	res := proto.GetTasksByStatusRep{}

	err := tc.RPC("TaskSrv.GetTasksByStatus", &arg, &res)
	return res.Ids, err
}

func (tc *RawFtTaskClnt) ReadTasks(ids []TaskId) ([]Task[[]byte], error) {
	arg := proto.ReadTasksReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.ReadTasksRep{}

	err := tc.RPC("TaskSrv.ReadTasks", &arg, &res)
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

	err := tc.RPC("TaskSrv.MoveTasks", &arg, &res)
	return err
}

func (tc *RawFtTaskClnt) MoveTasksByStatus(from, to TaskStatus) (int32, error) {
	arg := proto.MoveTasksByStatusReq{From: from, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksByStatusRep{}

	err := tc.RPC("TaskSrv.MoveTasksByStatus", &arg, &res)
	return res.NumMoved, err
}

func (tc *RawFtTaskClnt) GetTaskOutputs(ids []TaskId) ([][]byte, error) {
	arg := proto.GetTaskOutputsReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.GetTaskOutputsRep{}

	err := tc.RPC("TaskSrv.GetTaskOutputs", &arg, &res)
	if err != nil {
		return nil, err
	}

	return res.Outputs, nil
}

func (tc *RawFtTaskClnt) AddTaskOutputs(ids []TaskId, outputs [][]byte) error {
	arg := proto.AddTaskOutputsReq{Ids: ids, Outputs: outputs, Fence: tc.fenceProto()}
	res := proto.AddTaskOutputsRep{}

	err := tc.RPC("TaskSrv.AddTaskOutputs", &arg, &res)
	return err
}

func (tc *RawFtTaskClnt) AcquireTasks(wait bool) ([]TaskId, bool, error) {
	arg := proto.AcquireTasksReq{Wait: wait, Fence: tc.fenceProto()}
	res := proto.AcquireTasksRep{}

	err := tc.RPC("TaskSrv.AcquireTasks", &arg, &res)
	return res.Ids, res.Stopped, err
}

func (tc *RawFtTaskClnt) Stats() (*proto.TaskStats, error) {
	arg := proto.GetTaskStatsReq{}
	res := proto.GetTaskStatsRep{}

	err := tc.RPC("TaskSrv.GetTaskStats", &arg, &res)
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

	err := tc.RPC("TaskSrv.SubmitStop", &arg, &res)
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

	err := tc.RPC("TaskSrv.Fence", &arg, &res)
	return err
}

func (tc *RawFtTaskClnt) ClearEtcd() error {
	arg := proto.ClearEtcdReq{}
	res := proto.ClearEtcdRep{}

	err := tc.RPC("TaskSrv.ClearEtcd", &arg, &res)
	return err
}

func (tc *RawFtTaskClnt) Raw() FtTaskClnt[[]byte, []byte] {
	return tc
}

func (tc *RawFtTaskClnt) ServerId() task.FtTaskSrvId {
	return tc.serverId
}