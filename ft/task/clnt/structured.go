// Defines fttask client that encodes and decodes Go objects into bytes to store on the server
package clnt

import (
	"bytes"
	"encoding/json"
	"sigmaos/ft/task"
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

func NewFtTaskClnt[Data any, Output any](fsl *fslib.FsLib, serverId task.FtTaskSvcId, a *AcquireId) FtTaskClnt[Data, Output] {
	raw := newRawFtTaskClnt(fsl, serverId, a)
	tc := &ftTaskClnt[Data, Output]{
		raw,
	}
	return tc
}

func (tc *ftTaskClnt[Data, Output]) SubmitTasks(tasks []*Task[Data]) error {
	var raw_tasks []*Task[[]byte]
	for _, task := range tasks {
		encoded, err := Encode(task.Data)
		if err != nil {
			return err
		}
		raw_tasks = append(raw_tasks, &Task[[]byte]{
			Id:   task.Id,
			Data: encoded,
		})
	}
	return tc.RawFtTaskClnt.SubmitTasks(raw_tasks)
}

func (tc *ftTaskClnt[Data, Output]) EditTasks(tasks []*Task[Data]) ([]TaskId, error) {
	var raw_tasks []*Task[[]byte]
	for _, task := range tasks {
		encoded, err := Encode(task.Data)
		if err != nil {
			return nil, err
		}
		raw_tasks = append(raw_tasks, &Task[[]byte]{
			Id:   task.Id,
			Data: encoded,
		})
	}
	return tc.RawFtTaskClnt.EditTasks(raw_tasks)
}

func (tc *ftTaskClnt[Data, Output]) ReadTasks(ids []TaskId) ([]Task[Data], error) {
	raw_tasks, err := tc.RawFtTaskClnt.ReadTasks(ids)
	if err != nil {
		return nil, err
	}
	var tasks []Task[Data]
	for _, raw_task := range raw_tasks {
		var data Data
		err := json.NewDecoder(bytes.NewReader(raw_task.Data)).Decode(&data)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, Task[Data]{Id: raw_task.Id, Data: data})
	}
	return tasks, nil
}

func (tc *ftTaskClnt[Data, Output]) GetTaskOutputs(ids []TaskId) ([]Output, error) {
	raw_out, err := tc.RawFtTaskClnt.GetTaskOutputs(ids)
	if err != nil {
		return nil, err
	}
	outputs := make([]Output, len(ids))
	for ix, out := range raw_out {
		outputs[ix], err = Decode[Output](out)
		if err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

func (tc *ftTaskClnt[Data, Output]) AddTaskOutputs(ids []TaskId, outputs []Output, markDone bool) error {
	raw_out := make([][]byte, len(outputs))
	var err error
	for ix, output := range outputs {
		raw_out[ix], err = Encode(output)
		if err != nil {
			return err
		}
	}
	return tc.RawFtTaskClnt.AddTaskOutputs(ids, raw_out, markDone)
}

func (tc *ftTaskClnt[Data, Output]) AsRawClnt() FtTaskClnt[[]byte, []byte] {
	return tc.RawFtTaskClnt
}
