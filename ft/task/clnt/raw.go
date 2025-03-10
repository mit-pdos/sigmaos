// Defines fttask client that sends just tasks as raw bytes
package clnt

import (
	"fmt"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/ft/task"
	fttask "sigmaos/ft/task"
	"sigmaos/ft/task/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sync"
	"time"

	protobuf "google.golang.org/protobuf/proto"
)

type RawFtTaskClnt struct {
	rpcclntc        *rpcclnt.ClntCache
	fsl             *fslib.FsLib
	serverId        fttask.FtTaskSrvId
	currInstance    string
	fence           *sp.Tfence
	mu              sync.Mutex
}

func newRawFtTaskClnt(fsl *fslib.FsLib, serverId fttask.FtTaskSrvId) *RawFtTaskClnt {
	tc := &RawFtTaskClnt{
		fsl:          fsl,
		rpcclntc:     rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
		serverId:     serverId,
		currInstance: "",
		fence:        nil,
		mu:   			  sync.Mutex{},
	}
	return tc
}

func (tc *RawFtTaskClnt) fenceProto() *sp.TfenceProto {
	if tc.fence == nil {
		return nil
	}

	return tc.fence.FenceProto()
}

func (tc *RawFtTaskClnt) getAvailableInstances() ([]string, error) {
	instances, _, err := tc.fsl.ReadDir(tc.serverId.ServerPath())
	instanceIds := make([]string, 0)
	for _, instance := range instances {
		instanceIds = append(instanceIds, instance.Name)
	}

	return instanceIds, err
}

func (tc *RawFtTaskClnt) rpc(method string, arg protobuf.Message, res protobuf.Message, isPing bool) error {
	// lock to protect tc.currInstance
	tc.mu.Lock()
	currInstance := tc.currInstance
	tc.mu.Unlock()

	if currInstance != "" {
		serverPath := filepath.Join(tc.serverId.ServerPath(), currInstance)
		err := tc.rpcclntc.RPC(serverPath, method, arg, res)

		// if this is an error unrelated to finding the server, return it
		if !(serr.IsErrorUnavailable(err) && (err.(*serr.Err).Obj == serverPath || err.(*serr.Err).Obj == currInstance)) {
			return err
		}

		db.DPrintf(db.FTTASKS, "rpc to last known instance %s failed: %v", tc.currInstance, err)
	}

	numRetries := fttask.CLNT_NUM_RETRIES
	if isPing {
		numRetries = 5
	}

	var instances []string
	for i := 0; i < numRetries; i++ {
		var err error

		instances, err = tc.getAvailableInstances()
		if err != nil {
			return err
		}

		for _, instance := range instances {
			if instance == currInstance {
				continue
			}

			err := tc.rpcclntc.RPC(filepath.Join(tc.serverId.ServerPath(), instance), method, arg, res)
			if err == nil {
				tc.mu.Lock()
				tc.currInstance = instance
				tc.mu.Unlock()
				return nil
			}
			db.DPrintf(db.FTTASKS, "rpc %s to %s failed: %v", method, instance, err)
		}

		db.DPrintf(db.FTTASKS, "rpc %s to all instances %v failed", method, instances)
		time.Sleep(fttask.CLNT_RETRY_TIMEOUT)
	}
	db.DPrintf(db.FTTASKS, "rpc %s to all instances %v failed %d times", method, instances, numRetries)
	return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("no instances available, %v all failed", instances))
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

	err := tc.rpc("TaskSrv.SubmitTasks", &arg, &res, false)
	return res.Existing, err
}

func (tc *RawFtTaskClnt) GetTasksByStatus(taskStatus TaskStatus) ([]TaskId, error) {
	arg := proto.GetTasksByStatusReq{Status: taskStatus, Fence: tc.fenceProto()}
	res := proto.GetTasksByStatusRep{}

	err := tc.rpc("TaskSrv.GetTasksByStatus", &arg, &res, false)
	return res.Ids, err
}

func (tc *RawFtTaskClnt) ReadTasks(ids []TaskId) ([]Task[[]byte], error) {
	arg := proto.ReadTasksReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.ReadTasksRep{}

	err := tc.rpc("TaskSrv.ReadTasks", &arg, &res, false)
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

	err := tc.rpc("TaskSrv.MoveTasks", &arg, &res, false)
	return err
}

func (tc *RawFtTaskClnt) MoveTasksByStatus(from, to TaskStatus) (int32, error) {
	arg := proto.MoveTasksByStatusReq{From: from, To: to, Fence: tc.fenceProto()}
	res := proto.MoveTasksByStatusRep{}

	err := tc.rpc("TaskSrv.MoveTasksByStatus", &arg, &res, false)
	return res.NumMoved, err
}

func (tc *RawFtTaskClnt) GetTaskOutputs(ids []TaskId) ([][]byte, error) {
	arg := proto.GetTaskOutputsReq{Ids: ids, Fence: tc.fenceProto()}
	res := proto.GetTaskOutputsRep{}

	err := tc.rpc("TaskSrv.GetTaskOutputs", &arg, &res, false)
	if err != nil {
		return nil, err
	}

	return res.Outputs, nil
}

func (tc *RawFtTaskClnt) AddTaskOutputs(ids []TaskId, outputs [][]byte) error {
	arg := proto.AddTaskOutputsReq{Ids: ids, Outputs: outputs, Fence: tc.fenceProto()}
	res := proto.AddTaskOutputsRep{}

	err := tc.rpc("TaskSrv.AddTaskOutputs", &arg, &res, false)
	return err
}

func (tc *RawFtTaskClnt) AcquireTasks(wait bool) ([]TaskId, bool, error) {
	arg := proto.AcquireTasksReq{Wait: wait, Fence: tc.fenceProto()}
	res := proto.AcquireTasksRep{}

	err := tc.rpc("TaskSrv.AcquireTasks", &arg, &res, false)
	return res.Ids, res.Stopped, err
}

func (tc *RawFtTaskClnt) Stats() (*proto.TaskStats, error) {
	arg := proto.GetTaskStatsReq{}
	res := proto.GetTaskStatsRep{}

	err := tc.rpc("TaskSrv.GetTaskStats", &arg, &res, false)
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

	err := tc.rpc("TaskSrv.SubmitStop", &arg, &res, false)
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

	err := tc.rpc("TaskSrv.Fence", &arg, &res, false)
	return err
}

func (tc *RawFtTaskClnt) ClearEtcd() error {
	arg := proto.ClearEtcdReq{}
	res := proto.ClearEtcdRep{}

	err := tc.rpc("TaskSrv.ClearEtcd", &arg, &res, false)
	return err
}

// does a call to GetTaskStats since this is a simple status check that also bypasses fence
func (tc *RawFtTaskClnt) Ping() error {
	arg := proto.GetTaskStatsReq{}
	res := proto.GetTaskStatsRep{}

	// bypass the rpc wrapper to avoid retrying
	err := tc.rpc("TaskSrv.GetTaskStats", &arg, &res, true)
	return err
}

func (tc *RawFtTaskClnt) Partition() (string, error) {
	arg := proto.PartitionReq{}
	res := proto.PartitionRep{}

	err := tc.rpc("TaskSrv.Partition", &arg, &res, false)
	return tc.currInstance, err
}

func (tc *RawFtTaskClnt) AsRawClnt() FtTaskClnt[[]byte, []byte] {
	return tc
}

func (tc *RawFtTaskClnt) ServerId() task.FtTaskSrvId {
	return tc.serverId
}
