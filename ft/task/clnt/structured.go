// Defines fttask client that encodes and decodes Go objects into bytes to store on the server
package clnt

import (
	"bytes"
	"encoding/json"
	"sigmaos/ft/task"
	"sigmaos/ft/task/proto"
	"sigmaos/sigmaclnt/fslib"
)

type ftTaskClnt[Data any, Output any] struct {
	*RawFtTaskClnt
}

func Encode[T any](data T) ([]byte, error) {
	return json.Marshal(data)
}

func Decode[T any](encoded []byte) (T, error) {
	var data T
	err := json.Unmarshal(encoded, &data)
	return data, err
}

func NewFtTaskClnt[Data any, Output any](fsl *fslib.FsLib, serverId task.FtTaskSrvId) FtTaskClnt[Data, Output] {
	raw := newRawFtTaskClnt(fsl, serverId)
	tc := &ftTaskClnt[Data, Output]{
		raw,
	}
	return tc
}

func (tc *ftTaskClnt[Data, Output]) SubmitTasks(tasks []*Task[Data]) ([]TaskId, error) {
	var protoTasks []*proto.Task

	for _, task := range tasks {
		encoded, err := Encode(task.Data)
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

	err := tc.rpc("TaskSrv.SubmitTasks", &arg, &res)
	return res.Existing, err
}

func (tc *ftTaskClnt[Data, Output]) EditTasks(tasks []*Task[Data]) ([]TaskId, error) {
	var protoTasks []*proto.Task

	for _, task := range tasks {
		encoded, err := Encode(task.Data)
		if err != nil {
			return nil, err
		}

		protoTasks = append(protoTasks, &proto.Task{
			Id:   task.Id,
			Data: encoded,
		})
	}

	arg := proto.EditTasksReq{Tasks: protoTasks, Fence: tc.fenceProto()}
	res := proto.EditTasksRep{}

	err := tc.rpc("TaskSrv.EditTasks", &arg, &res)
	return res.Unknown, err
}

func (tc *ftTaskClnt[Data, Output]) ReadTasks(ids []TaskId) ([]Task[Data], error) {
	arg := proto.ReadTasksReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.ReadTasksRep{}

	err := tc.rpc("TaskSrv.ReadTasks", &arg, &res)
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

func (tc *ftTaskClnt[Data, Output]) GetTaskOutputs(ids []TaskId) ([]Output, error) {
	arg := proto.GetTaskOutputsReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.GetTaskOutputsRep{}

	err := tc.rpc("TaskSrv.GetTaskOutputs", &arg, &res)
	if err != nil {
		return nil, err
	}

	outputs := make([]Output, len(ids))
	for ix, output := range res.Outputs {
		outputs[ix], err = Decode[Output](output)

		if err != nil {
			return nil, err
		}
	}

	return outputs, nil
}

func (tc *ftTaskClnt[Data, Output]) AddTaskOutputs(ids []TaskId, outputs []Output, markDone bool) error {
	encoded := make([][]byte, len(outputs))
	var err error
	for ix, output := range outputs {
		encoded[ix], err = Encode(output)
		if err != nil {
			return err
		}
	}

	arg := proto.AddTaskOutputsReq{Ids: ids, Outputs: encoded, MarkDone: markDone, Fence: tc.fenceProto()}
	res := proto.AddTaskOutputsRep{}

	err = tc.rpc("TaskSrv.AddTaskOutputs", &arg, &res)
	return err
}

func (tc *ftTaskClnt[Data, Output]) AsRawClnt() FtTaskClnt[[]byte, []byte] {
	return tc.RawFtTaskClnt
}
