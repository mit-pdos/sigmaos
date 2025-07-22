package srv

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt/electclnt"
	fttask "sigmaos/ft/task"
	"sigmaos/ft/task/proto"
	"sigmaos/proc"
	"sigmaos/serr"
	sesssrv "sigmaos/session/srv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/crash"
)

const CRASHID = -1

type TaskSrv struct {
	data   map[int32][]byte
	output map[int32][]byte
	status map[int32]proto.TaskStatus

	todo    map[int32]bool
	wip     map[int32]bool
	done    map[int32]bool
	errored map[int32]bool

	mu                       *sync.Mutex
	todoCond                 *sync.Cond
	hasLastTaskBeenSubmitted bool
	fence                    *sp.Tfence
	rootId                   fttask.FtTaskSvcId

	etcdClient *clientv3.Client
	electclnt  *electclnt.ElectClnt
	sc         *sigmaclnt.SigmaClnt

	expired atomic.Bool
}

const (
	ETCD_STATUS                   = "status"
	ETCD_DATA                     = "data"
	ETCD_OUTPUT                   = "output"
	ETCD_SRV_FENCE                = "srv_fence"  // ensures only the most recently elected fttask srv can write to etcd
	ETCD_CLNT_FENCE               = "clnt_fence" // ensures only the most recently elected client can write to fttask srv
	ETCD_LASTTASKHASBEENSUBMITTED = "submittedLastTask"
)

const (
	// etcd limits the number of operations per transaction to 128 by default
	OPS_PER_TXN = 128
)

func (s *TaskSrv) root() string {
	return fmt.Sprintf("%s:/fttask/%s/", proc.GetProcEnv().GetRealm(), s.rootId.String())
}

func (s *TaskSrv) keyPrefix(prefix string) string {
	return s.root() + prefix + "/"
}

func (s *TaskSrv) key(taskId int32, prefix string) string {
	return s.keyPrefix(prefix) + strconv.FormatInt(int64(taskId), 10)
}

func RunTaskSrv(args []string) error {
	fttaskId := args[0]

	pe := proc.GetProcEnv()
	mu := &sync.Mutex{}
	s := &TaskSrv{
		mu:       mu,
		data:     make(map[int32][]byte),
		output:   make(map[int32][]byte),
		status:   make(map[int32]proto.TaskStatus),
		todo:     make(map[int32]bool),
		todoCond: sync.NewCond(mu),
		wip:      make(map[int32]bool),
		done:     make(map[int32]bool),
		errored:  make(map[int32]bool),
		rootId:   fttask.FtTaskSvcId(fttaskId),
	}

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return err
	}
	s.sc = sc

	db.DPrintf(db.FTTASKSRV, "Start: %v %v", args, sc)

	if err := s.sc.Started(); err != nil {
		return err
	}
	ch := make(chan error)
	go s.sc.WaitExitChan(ch)

	etcdMnts := pe.GetEtcdEndpoints()
	dial := s.sc.GetDialProxyClnt().Dial

	endpoints := []string{}
	for addr := range etcdMnts {
		endpoints = append(endpoints, addr)
	}

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			ep, ok := etcdMnts[addr]
			// Check that the endpoint is in the map
			if !ok {
				db.DFatalf("Unknown fsetcd endpoint proto: addr %v eps %v", addr, etcdMnts)
			}

			return dial(sp.NewEndpointFromProto(ep))
		})},
	})
	if err != nil {
		return err
	}
	s.etcdClient = etcdClient

	if err := s.acquireLeadership(); err != nil {
		return err
	}

	go func() {
		<-s.electclnt.Done()
		db.DPrintf(db.FTTASKSRV, "Session expired")
		s.expired.Store(true)
		s.electclnt.ReleaseLeadership()
		crash.Crash()
	}()

	if err := s.readEtcd(); err != nil {
		return err
	}

	ssrv, err := sigmasrv.NewSigmaSrvClnt(fttask.FtTaskSvcId(fttaskId).ServicePath(), s.sc, s, sesssrv.WithExp(s))
	if err != nil {
		return err
	}

	db.DPrintf(db.FTTASKSRV, "Created fttask srv %s", fttaskId)

	crash.SetCrashFile(s.sc.FsLib, crash.FTTASKSRV_SUBMITCRASH)

	crash.Failer(s.sc.FsLib, crash.FTTASKSRV_CRASH, func(e crash.Tevent) {
		crash.Crash()
	})

	crash.Failer(s.sc.FsLib, crash.FTTASKSRV_PARTITION, func(e crash.Tevent) {
		db.DPrintf(db.FTTASKSRV, "partition; delay %v", e.Delay)
		s.electclnt.Orphan()
	})

	err = <-ch

	db.DPrintf(db.FTTASKSRV, "task srv done %v err %v", fttaskId, err)

	if err := s.electclnt.ReleaseLeadership(); err != nil {
		return err
	}

	ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))

	return nil
}

func (s *TaskSrv) acquireLeadership() error {
	electclnt, err := electclnt.NewElectClnt(s.sc.FsLib, filepath.Join(sp.NAMED, "fttask", s.rootId.String()+"-leader"), sp.Tperm(0777))
	if err != nil {
		return err
	}
	s.electclnt = electclnt

	db.DPrintf(db.FTTASKSRV, "Acquiring leadership...")
	if err := electclnt.AcquireLeadership([]byte("")); err != nil {
		return err
	}

	db.DPrintf(db.FTTASKSRV, "Acquired leadership with fence %v", s.electclnt.Fence())

	// create server fence epoch if it doesn't exist
	_, _ = s.etcdClient.Txn(context.TODO()).If(
		clientv3.Compare(clientv3.CreateRevision(s.root()+ETCD_SRV_FENCE), "=", 0),
	).Then(
		clientv3.OpPut(s.root()+ETCD_SRV_FENCE, "-1"),
	).Commit()

	// write new fence to etcd with election results
	resp, err := s.etcdClient.Txn(context.TODO()).If(
		clientv3.Compare(clientv3.Value(s.root()+ETCD_SRV_FENCE), "<", strconv.FormatUint(uint64(s.electclnt.Fence().Epoch), 10)),
	).Then(
		clientv3.OpPut(s.root()+ETCD_SRV_FENCE, strconv.FormatInt(int64(s.electclnt.Fence().Epoch), 10)),
	).Commit()

	if err != nil {
		db.DPrintf(db.FTTASKSRV, "Failed to write fence to etcd upon election %+v", err)
		return err
	}

	if !resp.Succeeded {
		db.DPrintf(db.FTTASKSRV, "Failed to write fence to etcd upon election with err = nil, %v", resp)
		return serr.NewErr(serr.TErrError, "etcd txn failed")
	}

	return nil
}

func (s *TaskSrv) Expired() bool {
	b := s.expired.Load()
	if b {
		db.DPrintf(db.FTTASKSRV, "Reject request; lease expired")
	}
	return b
}

func (s *TaskSrv) parseStatus(val string) (proto.TaskStatus, error) {
	switch val {
	case proto.TaskStatus_TODO.String():
		return proto.TaskStatus_TODO, nil
	case proto.TaskStatus_WIP.String():
		return proto.TaskStatus_WIP, nil
	case proto.TaskStatus_DONE.String():
		return proto.TaskStatus_DONE, nil
	case proto.TaskStatus_ERROR.String():
		return proto.TaskStatus_ERROR, nil
	default:
		return proto.TaskStatus_TODO, serr.NewErr(serr.TErrInval, val)
	}
}

func (s *TaskSrv) readEtcd() error {
	resp, err := s.etcdClient.Get(context.TODO(), s.root(), clientv3.WithPrefix())
	if err != nil {
		return err
	}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		val := string(kv.Value)
		db.DPrintf(db.FTTASKSRV, "ReadEtcd: key %v val %v", key, val)

		if strings.HasPrefix(key, s.keyPrefix(ETCD_STATUS)) {
			id, err := strconv.ParseInt(strings.TrimPrefix(key, s.keyPrefix(ETCD_STATUS)), 10, 32)
			if err != nil {
				return err
			}

			status, err := s.parseStatus(val)
			if err != nil {
				return err
			}

			s.status[int32(id)] = status
			(*s.getMap(status))[int32(id)] = true
		} else if strings.HasPrefix(key, s.keyPrefix(ETCD_DATA)) {
			id, err := strconv.ParseInt(strings.TrimPrefix(key, s.keyPrefix(ETCD_DATA)), 10, 32)
			if err != nil {
				return err
			}

			s.data[int32(id)] = []byte(val)
		} else if strings.HasPrefix(key, s.keyPrefix(ETCD_OUTPUT)) {
			id, err := strconv.ParseInt(strings.TrimPrefix(key, s.keyPrefix(ETCD_OUTPUT)), 10, 32)
			if err != nil {
				return err
			}

			s.output[int32(id)] = []byte(val)
		} else if key == s.root()+ETCD_SRV_FENCE {
			// ignore
		} else if key == s.root()+ETCD_CLNT_FENCE {
			epoch, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}

			s.fence = &sp.Tfence{Epoch: sp.Tepoch(epoch)}
		} else if key == s.root()+ETCD_LASTTASKHASBEENSUBMITTED {
			s.hasLastTaskBeenSubmitted = true
		} else {
			db.DPrintf(db.ERROR, "Unknown key type %v", key)
			return serr.NewErr(serr.TErrInval, key)
		}
	}

	return nil
}

// must hold lock
func (s *TaskSrv) getMap(status proto.TaskStatus) *map[int32]bool {
	switch status {
	case proto.TaskStatus_TODO:
		return &s.todo
	case proto.TaskStatus_WIP:
		return &s.wip
	case proto.TaskStatus_DONE:
		return &s.done
	case proto.TaskStatus_ERROR:
		return &s.errored
	default:
		db.DPrintf(db.ERROR, "Invalid status: %v", status)
		return nil
	}
}

// must hold lock
func (s *TaskSrv) applyChanges(added map[int32]bool, status map[int32]proto.TaskStatus, data map[int32][]byte, outputs map[int32][]byte) error {
	if len(added)+len(status)+len(data)+len(outputs) == 0 {
		return nil
	}

	// validate changes
	for id, newStatus := range status {
		from, ok := s.status[id]
		if !ok {
			return serr.NewErr(serr.TErrNotfound, id)
		}
		to := newStatus

		if from == to {
			continue
		}

		fromMap := s.getMap(from)
		toMap := s.getMap(to)
		if fromMap == nil || toMap == nil {
			return serr.NewErr(serr.TErrInval, fmt.Sprintf("invalid status %v -> %v", from, to))
		}

		// we already checked if from == to so if this is true, then something is wrong with our server state
		if (*toMap)[id] {
			db.DFatalf("Task %v already in %v when trying to move it there", id, to)
		}
	}

	// write changes to etcd
	ops := make([]clientv3.Op, 0, OPS_PER_TXN)

	commitTxn := func() error {
		db.DPrintf(db.FTTASKSRV, "WriteChanges: writing to db with %d ops", len(ops))
		resp, err := s.etcdClient.Txn(context.TODO()).If(
			clientv3.Compare(clientv3.Value(s.root()+ETCD_SRV_FENCE), "=", strconv.FormatUint(uint64(s.electclnt.Fence().Epoch), 10)),
		).Then(ops...).Commit()

		if err != nil {
			db.DPrintf(db.FTTASKSRV, "WriteChanges: error with txn %v", err)
			return err
		}
		if !resp.Succeeded {
			db.DPrintf(db.ERROR, "WriteChanges: txn failed %v", resp)
			return serr.NewErr(serr.TErrError, "etcd txn failed")
		}
		db.DPrintf(db.FTTASKSRV, "WriteChanges: wrote to db with resp %v", resp)
		return nil
	}

	addOp := func(op clientv3.Op) error {
		ops = append(ops, op)
		if len(ops) >= OPS_PER_TXN {
			if err := commitTxn(); err != nil {
				// return early if a txn fails
				return err
			}
			// reset slice for the next batch
			ops = ops[:0]
		}

		return nil
	}

	for id := range added {
		if err := addOp(clientv3.OpPut(s.key(id, ETCD_STATUS), proto.TaskStatus_TODO.String())); err != nil {
			return err
		}
	}

	for id, newStatus := range status {
		if err := addOp(clientv3.OpPut(s.key(id, ETCD_STATUS), newStatus.String())); err != nil {
			return err
		}
	}

	for id, d := range data {
		if err := addOp(clientv3.OpPut(s.key(id, ETCD_DATA), string(d))); err != nil {
			return err
		}
	}

	for id, output := range outputs {
		if err := addOp(clientv3.OpPut(s.key(id, ETCD_OUTPUT), string(output))); err != nil {
			return err
		}
	}

	// commit any remaining operations
	if len(ops) > 0 {
		if err := commitTxn(); err != nil {
			return err
		}
	}

	// update local cache
	for id := range added {
		s.status[id] = proto.TaskStatus_TODO
		(*s.getMap(proto.TaskStatus_TODO))[id] = true
	}

	for id, newStatus := range status {
		from := s.status[id]
		to := newStatus
		if from != to {
			s.status[id] = to
			delete(*s.getMap(from), id)
			(*s.getMap(to))[id] = true
		}
	}

	for id, d := range data {
		s.data[id] = d
	}

	for id, output := range outputs {
		s.output[id] = output
	}

	s.todoCond.Broadcast()

	db.DPrintf(db.FTTASKSRV, "WriteChanges: wrote changes to local cache")

	return nil
}

// must hold lock
func (s *TaskSrv) get(status proto.TaskStatus) []int32 {
	m := s.getMap(status)
	if m == nil {
		return nil
	}

	ret := make([]int32, 0)
	for k := range *m {
		ret = append(ret, k)
	}

	return ret
}

// must hold lock
func (s *TaskSrv) checkFence(fence *sp.TfenceProto) error {
	if s.fence == nil {
		return nil
	}

	if fence == nil || s.fence.Epoch > fence.Tfence().Epoch {
		db.DPrintf(db.FTTASKSRV, "checkFence: failed curr: %v req: %v", s.fence, fence)
		return serr.NewErr(serr.TErrInval, fmt.Sprintf("fence %v is not after %v", fence, s.fence))
	}

	return nil
}

func (s *TaskSrv) SubmitTasks(ctx fs.CtxI, req proto.SubmitTasksReq, rep *proto.SubmitTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	existing := make([]int32, 0)
	for _, task := range req.Tasks {
		if _, ok := s.data[task.Id]; ok {
			existing = append(existing, task.Id)
		}
	}

	added := make(map[int32]bool)
	data := make(map[int32][]byte)
	for _, task := range req.Tasks {
		added[task.Id] = true
		data[task.Id] = task.Data
	}

	if err := s.applyChanges(added, nil, data, nil); err != nil {
		return err
	}

	rep.Existing = existing

	if len(req.Tasks) > 0 {
		crash.CrashFile(strconv.Itoa(int(req.Tasks[0].Id)))
	}

	db.DPrintf(db.FTTASKSRV, "SubmitTasks: total: %d, exist: %d", len(req.Tasks), len(existing))

	return nil
}

func (s *TaskSrv) EditTasks(ctx fs.CtxI, req proto.EditTasksReq, rep *proto.EditTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	unknown := make([]int32, 0)
	for _, task := range req.Tasks {
		if _, ok := s.data[task.Id]; !ok {
			unknown = append(unknown, task.Id)
		}
	}

	data := make(map[int32][]byte)
	for _, task := range req.Tasks {
		if _, ok := s.data[task.Id]; ok {
			data[task.Id] = task.Data
		}
	}

	if err := s.applyChanges(nil, nil, data, nil); err != nil {
		return err
	}

	rep.Unknown = unknown

	db.DPrintf(db.FTTASKSRV, "EditTasks: total: %d, unknown: %d", len(req.Tasks), len(unknown))

	return nil
}

func (s *TaskSrv) GetTasksByStatus(ctx fs.CtxI, req proto.GetTasksByStatusReq, rep *proto.GetTasksByStatusRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	rep.Ids = s.get(req.Status)

	if rep.Ids == nil {
		return serr.NewErr(serr.TErrInval, req.Status)
	}

	db.DPrintf(db.FTTASKSRV, "GetTasksByStatus: %v n: %d", req.Status, len(rep.Ids))
	return nil
}

func (s *TaskSrv) ReadTasks(ctx fs.CtxI, req proto.ReadTasksReq, rep *proto.ReadTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	rep.Tasks = make([]*proto.Task, 0)
	for _, id := range req.Ids {
		if _, ok := s.data[id]; !ok {
			return serr.NewErr(serr.TErrNotfound, id)
		}

		rep.Tasks = append(rep.Tasks, &proto.Task{
			Id:   id,
			Data: s.data[id],
		})
	}

	db.DPrintf(db.FTTASKSRV, "ReadTasks: n: %d", len(rep.Tasks))

	return nil
}

func (s *TaskSrv) MoveTasks(ctx fs.CtxI, req proto.MoveTasksReq, rep *proto.MoveTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	status := make(map[int32]proto.TaskStatus)
	for _, id := range req.Ids {
		status[id] = req.To
	}

	if err := s.applyChanges(nil, status, nil, nil); err != nil {
		return err
	}

	db.DPrintf(db.FTTASKSRV, "MoveTasks: n: %d, to: %v", len(req.Ids), req.To)

	return nil
}

func (s *TaskSrv) MoveTasksByStatus(ctx fs.CtxI, req proto.MoveTasksByStatusReq, rep *proto.MoveTasksByStatusRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	from := s.getMap(req.From)
	if from == nil {
		return serr.NewErr(serr.TErrInval, req.From)
	}

	db.DPrintf(db.FTTASKSRV, "MoveTasksByStatus: %v, from: %v, to: %v", *from, req.From, req.To)

	status := make(map[int32]proto.TaskStatus)
	for id := range *from {
		status[id] = req.To
	}

	db.DPrintf(db.FTTASKSRV, "MoveTasksByStatus: %v, from: %v, to: %v", *from, req.From, req.To)

	n := len(*from)

	if err := s.applyChanges(nil, status, nil, nil); err != nil {
		return err
	}

	rep.NumMoved = int32(n)

	db.DPrintf(db.FTTASKSRV, "MoveTasksByStatus: n: %d, from: %v, to: %v", n, req.From, req.To)

	return nil
}

func (s *TaskSrv) GetTaskOutputs(ctx fs.CtxI, req proto.GetTaskOutputsReq, rep *proto.GetTaskOutputsRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	rep.Outputs = make([][]byte, len(req.Ids))
	for ix, id := range req.Ids {
		output, ok := s.output[id]
		if ok {
			rep.Outputs[ix] = output
		} else {
			return serr.NewErr(serr.TErrNotfound, id)
		}
	}

	return nil
}

func (s *TaskSrv) AddTaskOutputs(ctx fs.CtxI, req proto.AddTaskOutputsReq, rep *proto.AddTaskOutputsRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	outputs := make(map[int32][]byte)
	for ix, id := range req.Ids {
		outputs[id] = req.Outputs[ix]
	}

	status := make(map[int32]proto.TaskStatus)
	if req.MarkDone {
		for _, id := range req.Ids {
			status[id] = proto.TaskStatus_DONE
		}
	}

	if err := s.applyChanges(nil, status, nil, outputs); err != nil {
		return err
	}

	return nil
}

// caller should hold lock
func (s *TaskSrv) allTasksDone() bool {
	return len(s.wip) == 0 && len(s.todo) == 0 && s.hasLastTaskBeenSubmitted
}

func (s *TaskSrv) AcquireTasks(ctx fs.CtxI, req proto.AcquireTasksReq, rep *proto.AcquireTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	db.DPrintf(db.TEST, "acquireid: %v", req.AcquireId)

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	fence := s.fence
	ids := s.get(proto.TaskStatus_TODO)
	for req.Wait && len(ids) == 0 && !s.allTasksDone() && fence == s.fence {
		db.DPrintf(db.FTTASKSRV, "AcquireTasks: waiting for tasks...")
		s.todoCond.Wait()
		ids = s.get(proto.TaskStatus_TODO)
	}

	if fence != s.fence {
		return serr.NewErr(serr.TErrInval, fmt.Sprintf("fence changed from %v to %v", fence, s.fence))
	}

	status := make(map[int32]proto.TaskStatus)
	for _, id := range ids {
		status[id] = proto.TaskStatus_WIP
	}

	if err := s.applyChanges(nil, status, nil, nil); err != nil {
		return err
	}

	rep.Stopped = s.allTasksDone()
	rep.Ids = ids

	if len(ids) > 0 {
		crash.CrashFile(strconv.Itoa(int(ids[0])))
	}

	db.DPrintf(db.FTTASKSRV, "AcquireTasks: n: %d stopped: %t", len(rep.Ids), rep.Stopped)

	return nil
}

func (s *TaskSrv) GetTaskStats(ctx fs.CtxI, req proto.GetTaskStatsReq, rep *proto.GetTaskStatsRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rep.Stats = &proto.TaskStats{
		NumTodo:  int32(len(s.todo)),
		NumWip:   int32(len(s.wip)),
		NumDone:  int32(len(s.done)),
		NumError: int32(len(s.errored)),
	}

	db.DPrintf(db.FTTASKSRV, "GetTaskStats: %v", rep.Stats)

	return nil
}

func (s *TaskSrv) SubmittedLastTask(ctx fs.CtxI, req proto.SubmittedLastTaskReq, rep *proto.SubmittedLastTaskRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkFence(req.Fence); err != nil {
		return err
	}

	db.DPrintf(db.FTTASKSRV, "stop received")

	resp, err := s.etcdClient.Txn(context.TODO()).If(
		clientv3.Compare(clientv3.Value(s.root()+ETCD_SRV_FENCE), "=", strconv.FormatUint(uint64(s.electclnt.Fence().Epoch), 10)),
	).Then(
		clientv3.OpPut(s.root()+ETCD_LASTTASKHASBEENSUBMITTED, "1"),
	).Commit()

	if err != nil {
		db.DPrintf(db.FTTASKSRV, "Fence: error writing to etcd %v", err)
		return err
	}

	if !resp.Succeeded {
		db.DPrintf(db.FTTASKSRV, "Fence: txn failed %v", resp)
		return serr.NewErr(serr.TErrError, "etcd txn failed")
	}

	s.hasLastTaskBeenSubmitted = true
	s.todoCond.Broadcast()

	return nil
}

func (s *TaskSrv) Fence(ctx fs.CtxI, req proto.FenceReq, rep *proto.FenceRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fence := req.Fence.Tfence()
	if s.fence != nil && s.fence.Epoch >= fence.Epoch {
		return serr.NewErr(serr.TErrInval, fmt.Sprintf("fence already set with epoch %v, attempted to set with %v", fence.Epoch, s.fence.Epoch))
	}

	resp, err := s.etcdClient.Txn(context.TODO()).If(
		clientv3.Compare(clientv3.Value(s.root()+ETCD_SRV_FENCE), "=", strconv.FormatUint(uint64(s.electclnt.Fence().Epoch), 10)),
	).Then(
		clientv3.OpPut(s.root()+ETCD_CLNT_FENCE, strconv.FormatUint(req.Fence.Epoch, 10)),
	).Commit()

	if err != nil {
		db.DPrintf(db.FTTASKSRV, "Fence: error writing to etcd %v", err)
		return err
	}

	if !resp.Succeeded {
		db.DPrintf(db.FTTASKSRV, "Fence: txn failed %v", resp)
		return serr.NewErr(serr.TErrError, "etcd txn failed")
	}

	s.fence = &fence

	db.DPrintf(db.FTTASKSRV, "fence: added fence %v to replace %v", fence, s.fence)

	return nil
}

func (s *TaskSrv) ClearEtcd(ctx fs.CtxI, req proto.ClearEtcdReq, rep *proto.ClearEtcdRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	db.DPrintf(db.FTTASKSRV, "Shutdown: shutting down")
	resp, err := s.etcdClient.Txn(context.TODO()).
		If(clientv3.Compare(clientv3.Value(s.root()+ETCD_SRV_FENCE), "=", strconv.FormatUint(uint64(s.electclnt.Fence().Epoch), 10))).
		Then(clientv3.OpDelete(s.root(), clientv3.WithPrefix())).
		Commit()
	if err != nil {
		db.DPrintf(db.FTTASKSRV, "Shutdown: error deleting keys %v", err)
		return err
	}

	if !resp.OpResponse().Txn().Succeeded {
		db.DPrintf(db.FTTASKSRV, "Shutdown: txn failed %v", resp)
		return serr.NewErr(serr.TErrError, "etcd txn failed")
	}

	db.DPrintf(db.FTTASKSRV, "Shutdown: deleted keys %v", resp)

	return nil
}
