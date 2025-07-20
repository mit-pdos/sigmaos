// Defines client interface for interacting with fault tolerant task
// server. Clients shouldn't reuse TaskId's.
package clnt

import (
	db "sigmaos/debug"
	"sigmaos/ft/task"
	"sigmaos/ft/task/proto"
	sp "sigmaos/sigmap"

	protobuf "google.golang.org/protobuf/proto"
)

type TaskStatus = proto.TaskStatus
type TaskId = int32

const (
	TODO  = proto.TaskStatus_TODO
	WIP   = proto.TaskStatus_WIP
	DONE  = proto.TaskStatus_DONE
	ERROR = proto.TaskStatus_ERROR
)

type Task[Data any] struct {
	Id   TaskId
	Data Data
}

type FtTaskClnt[Data any, Output any] interface {
	SubmitTasks(tasks []*Task[Data]) ([]TaskId, error)
	SubmittedLastTask() error
	EditTasks(tasks []*Task[Data]) ([]TaskId, error)
	GetTasksByStatus(taskStatus TaskStatus) ([]TaskId, error)
	ReadTasks(ids []TaskId) ([]Task[Data], error)
	MoveTasks(ids []TaskId, to TaskStatus) error
	MoveTasksByStatus(from, to TaskStatus) (int32, error)
	GetTaskOutputs(ids []TaskId) ([]Output, error)
	AddTaskOutputs(ids []TaskId, outputs []Output, markDone bool) error
	AcquireTasks(wait bool) ([]TaskId, bool, error)
	Stats() (*proto.TaskStats, error)
	GetNTasks(status TaskStatus) (int32, error)
	SetFence(fence *sp.Tfence)
	GetFence() *sp.Tfence
	Fence(fence *sp.Tfence) error
	ClearEtcd() error
	AsRawClnt() FtTaskClnt[[]byte, []byte]
	ServiceId() task.FtTaskSvcId

	rpc(method string, arg protobuf.Message, res protobuf.Message) error
}

func GetTasks[Data any, Output any](tc FtTaskClnt[Data, Output], chTask chan []TaskId) {
	for {
		tasks, stopped, err := tc.AcquireTasks(true)
		if err != nil {
			db.DFatalf("AcquireTasks err %v", err)
		}
		db.DPrintf(db.FTTASKMGR, "GetTasks: AcquireTasks %v %t err %v", tasks, stopped, err)
		if len(tasks) != 0 {
			chTask <- tasks
		}
		if stopped {
			close(chTask)
			break
		}
	}
}
