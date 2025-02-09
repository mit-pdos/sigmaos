package clnt

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sigmaos/ft/task/proto"
	fttask_srv "sigmaos/ft/task/srv"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type TaskStatus = proto.TaskStatus
type TaskId = int32

const (
	TODO = proto.TaskStatus_TODO
	WIP = proto.TaskStatus_WIP
	DONE = proto.TaskStatus_DONE
	ERROR = proto.TaskStatus_ERROR
)

type FtTaskClnt[Data any, Output any] struct {
	rpc         *rpcclnt.ClntCache
	fsl         *fslib.FsLib
	serverPath  string
	ServerId    fttask_srv.FtTaskSrvId
	fence       *sp.Tfence
}

type Task[Data any] struct {
	Id   TaskId
	Data Data
}

func encode[T any] (data T) ([]byte, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)

	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decode[T any] (encoded []byte) (T, error) {
	var data T
	err := json.NewDecoder(bytes.NewReader(encoded)).Decode(&data)
	
	return data, err
}

func NewFtTaskClnt[Data any, Output any](fsl *fslib.FsLib, serverId fttask_srv.FtTaskSrvId) *FtTaskClnt[Data, Output] {
	tc := &FtTaskClnt[Data, Output]{
		fsl:       fsl,
		rpc: rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
		serverPath: filepath.Join(sp.NAMED, "fttask", string(serverId)),
		ServerId: serverId,
	}
	return tc
}

func (tc *FtTaskClnt[Data, Output]) fenceProto() *sp.TfenceProto {
	if tc.fence == nil {
		return nil
	}

	return tc.fence.FenceProto()
}

func (tc *FtTaskClnt[Data, Output]) SubmitTasks(tasks []*Task[Data]) ([]TaskId, error) {
	var protoTasks []*proto.Task

	for _, task := range tasks {
		encoded, err := encode(task.Data)
		if err != nil {
			return nil, err
		}

		protoTasks = append(protoTasks, &proto.Task{
			Id:   task.Id,
			Data: encoded,
		})
	}

	arg := proto.SubmitTasksReq{Tasks: protoTasks, Fence: tc.fenceProto()}
	res := proto.SubmitTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.SubmitTasks", &arg, &res)
	return res.Existing, err
}

func (tc *FtTaskClnt[Data, Output]) GetTasksByStatus(taskStatus TaskStatus) ([]TaskId, error) {
	arg := proto.GetTasksByStatusReq{Status: taskStatus, Fence: tc.fenceProto()}
	res := proto.GetTasksByStatusRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTasksByStatus", &arg, &res)
	return res.Ids, err
}

func (tc *FtTaskClnt[Data, Output]) ReadTasks(ids []TaskId) ([]Task[Data], error) {
	arg := proto.ReadTasksReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.ReadTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.ReadTasks", &arg, &res)
	if err != nil {
		return nil, err
	}

	var tasks []Task[Data]

	for _, protoTask := range res.Tasks {
		var data Data
		err := json.NewDecoder(bytes.NewReader(protoTask.Data)).Decode(&data)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, Task[Data]{Id: protoTask.Id, Data: data})
	}

	return tasks, nil
}

func (tc *FtTaskClnt[Data, Output]) MoveTasks(ids []TaskId, to TaskStatus) error {
	arg := proto.MoveTasksReq{Ids: ids, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.MoveTasks", &arg, &res)
	return err
}

func (tc *FtTaskClnt[Data, Output]) MoveTasksByStatus(from, to TaskStatus) error {
	arg := proto.MoveTasksByStatusReq{From: from, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksByStatusRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.MoveTasksByStatus", &arg, &res)
	return err
}

func (tc *FtTaskClnt[Data, Output]) GetTaskOutputs(ids []TaskId) ([]Output, error) {
	arg := proto.GetTaskOutputsReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.GetTaskOutputsRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTaskOutputs", &arg, &res)
	if err != nil {
		return nil, err
	}

	outputs := make([]Output, len(ids))
	for ix, output := range res.Outputs {
		outputs[ix], err = decode[Output](output)

		if err != nil {
			return nil, err
		}
	}

	return outputs, nil
}

func (tc *FtTaskClnt[Data, Output]) AddTaskOutputs(ids []TaskId, outputs []Output) error {
	encoded := make([][]byte, len(outputs))
	var err error
	for ix, output := range outputs {
		encoded[ix], err = encode(output)
		if err != nil {
			return err
		}
	}

	arg := proto.AddTaskOutputsReq{Ids: ids, Outputs: encoded, Fence: tc.fenceProto()}
	res := proto.AddTaskOutputsRep{}

	err = tc.rpc.RPC(tc.serverPath, "TaskSrv.AddTaskOutputs", &arg, &res)
	return err
}

func (tc *FtTaskClnt[Data, Output]) AcquireTasks(wait bool) ([]TaskId, bool, error) {
	arg := proto.AcquireTasksReq{Wait: wait, Fence: tc.fenceProto()}
	res := proto.AcquireTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.AcquireTasks", &arg, &res)
	return res.Ids, res.Stopped, err
}

func (tc *FtTaskClnt[Data, Output]) Stats() (*proto.TaskStats, error) {
	arg := proto.GetTaskStatsReq{}
	res := proto.GetTaskStatsRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTaskStats", &arg, &res)
	return res.Stats, err
}

func (tc *FtTaskClnt[Data, Output]) GetNTasks(status TaskStatus) (int32, error) {
	stats, err := tc.Stats()
	if err != nil {
		return 0, err
	}

	switch status {
	case TODO:
		return stats.NumTodo, nil
	case WIP:
		return stats.NumTodo, nil
	case DONE:
		return stats.NumTodo, nil
	case ERROR:
		return stats.NumTodo, nil
	default:
		return 0, serr.NewErr(serr.TErrInval, "invalid task status")
	}
}

func (tc *FtTaskClnt[Data, Output]) SubmitStop() error {
	arg := proto.SubmitStopReq{Fence: tc.fenceProto()}
	res := proto.SubmitStopRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.SubmitStop", &arg, &res)
	return err
}

func (tc *FtTaskClnt[Data, Output]) SetFence(fence *sp.Tfence) {
	tc.fence = fence
}

func (tc *FtTaskClnt[Data, Output]) GetFence() *sp.Tfence {
	return tc.fence
}

func (tc *FtTaskClnt[Data, Output]) Fence(fence *sp.Tfence) error {
	tc.SetFence(fence)

	arg := proto.FenceReq{Fence: tc.fenceProto()}
	res := proto.FenceRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.Fence", &arg, &res)
	return err
}