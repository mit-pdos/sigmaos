package clnt

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sigmaos/ft/task/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type TaskClnt[Data any, Output any] struct {
	rpc         *rpcclnt.ClntCache
	fsl         *fslib.FsLib
	serverPath  string
}

type Task[Data any] struct {
	Id   int32
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

func NewTaskClnt[Data any, Output any](fsl *fslib.FsLib, serverId string) *TaskClnt[Data, Output] {
	tc := &TaskClnt[Data, Output]{
		fsl:       fsl,
		rpc: rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
		serverPath: filepath.Join(sp.NAMED, "fttask", serverId),
	}
	return tc
}

func (tc *TaskClnt[Data, Output]) SubmitTasks(tasks []*Task[Data]) ([]int32, error) {
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

	arg := proto.SubmitTasksReq{Tasks: protoTasks}
	res := proto.SubmitTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.SubmitTasks", &arg, &res)
	return res.Existing, err
}

func (tc *TaskClnt[Data, Output]) GetTasksByStatus(taskStatus proto.TaskStatus) ([]int32, error) {
	arg := proto.GetTasksByStatusReq{Status: taskStatus}
	res := proto.GetTasksByStatusRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTasksByStatus", &arg, &res)
	return res.Ids, err
}

func (tc *TaskClnt[Data, Output]) ReadTasks(ids []int32) ([]Task[Data], error) {
	arg := proto.ReadTasksReq{Ids: ids}
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

func (tc *TaskClnt[Data, Output]) MoveTasks(ids []int32, to proto.TaskStatus) error {
	arg := proto.MoveTasksReq{Ids: ids, To: to}
	res := proto.MoveTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.MoveTasks", &arg, &res)
	return err
}

func (tc *TaskClnt[Data, Output]) MoveTasksByStatus(from, to proto.TaskStatus) error {
	arg := proto.MoveTasksByStatusReq{From: from, To: to}
	res := proto.MoveTasksByStatusRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.MoveTasksByStatus", &arg, &res)
	return err
}

func (tc *TaskClnt[Data, Output]) GetTaskOutputs(ids []int32) ([]Output, error) {
	arg := proto.GetTaskOutputsReq{Ids: ids}
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

func (tc *TaskClnt[Data, Output]) AddTaskOutputs(ids []int32, outputs []Output) error {
	encoded := make([][]byte, len(outputs))
	var err error
	for ix, output := range outputs {
		encoded[ix], err = encode(output)
		if err != nil {
			return err
		}
	}

	arg := proto.AddTaskOutputsReq{Ids: ids, Outputs: encoded}
	res := proto.AddTaskOutputsRep{}

	err = tc.rpc.RPC(tc.serverPath, "TaskSrv.AddTaskOutputs", &arg, &res)
	return err
}

func (tc *TaskClnt[Data, Output]) AcquireTasks(wait bool) ([]int32, error) {
	arg := proto.AcquireTasksReq{Wait: wait}
	res := proto.AcquireTasksRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.AcquireTasks", &arg, &res)
	return res.Ids, err
}

func (tc *TaskClnt[Data, Output]) Stats() (*proto.TaskStats, error) {
	arg := proto.GetTaskStatsReq{}
	res := proto.GetTaskStatsRep{}

	err := tc.rpc.RPC(tc.serverPath, "TaskSrv.GetTaskStats", &arg, &res)
	return res.Stats, err
}
