package srv

import (
	"path/filepath"
	"sigmaos/api/fs"
	"sigmaos/ft/task/proto"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmasrv"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type TaskSrv struct {
	mu       *sync.Mutex
	data     map[int32][]byte
	output   map[int32][]byte
	status   map[int32]proto.TaskStatus

	todo     map[int32]bool
	todoCond *sync.Cond
	wip      map[int32]bool
	done     map[int32]bool
	errored  map[int32]bool
}

func RunTaskSrv(args []string) error {
	pe := proc.GetProcEnv()
	mu := &sync.Mutex{}
	s := &TaskSrv{
		mu: mu,
		data: make(map[int32][]byte),
		output: make(map[int32][]byte),
		status: make(map[int32]proto.TaskStatus),
		todo: make(map[int32]bool),
		todoCond: sync.NewCond(mu),
		wip: make(map[int32]bool),
		done: make(map[int32]bool),
		errored: make(map[int32]bool),
	}

	id := args[0]

	ssrv, err := sigmasrv.NewSigmaSrv(filepath.Join(sp.NAMED, "fttask", id), s, pe)
	if err != nil {
		return err
	}

	db.DPrintf(db.FTTASKS, "Created fttask srv with args %v", args)

	return ssrv.RunServer()
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

func (s *TaskSrv) SubmitTasks(ctx fs.CtxI, req proto.SubmitTasksReq, rep *proto.SubmitTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := make([]int32, 0)
	for _, task := range req.Tasks {
		if _, ok := s.data[task.Id]; ok {
			existing = append(existing, task.Id)
		}
	}

	for _, task := range req.Tasks {
		s.todo[task.Id] = true
		s.data[task.Id] = task.Data
		s.status[task.Id] = proto.TaskStatus_TODO
	}

	rep.Existing = existing
	s.todoCond.Broadcast()

	return nil
}

func (s *TaskSrv) GetTasksByStatus(ctx fs.CtxI, req proto.GetTasksByStatusReq, rep *proto.GetTasksByStatusRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rep.Ids = s.get(req.Status)

	if rep.Ids == nil {
		return serr.NewErr(serr.TErrInval, req.Status)
	}
	return nil
}

func (s *TaskSrv) ReadTasks(ctx fs.CtxI, req proto.ReadTasksReq, rep *proto.ReadTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rep.Tasks = make([]*proto.Task, 0)
	for _, id := range req.Ids {
		if _, ok := s.data[id]; !ok {
			return serr.NewErr(serr.TErrNotfound, id)
		}

		rep.Tasks = append(rep.Tasks, &proto.Task{
			Id: id,
			Data: s.data[id],
		})
	}

	return nil
}

func (s *TaskSrv) MoveTasks(ctx fs.CtxI, req proto.MoveTasksReq, rep *proto.MoveTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	to := s.getMap(req.To)
	if to == nil {
		return serr.NewErr(serr.TErrInval, req.To)
	}

	for _, id := range req.Ids {
		if _, ok := s.status[id]; !ok {
			return serr.NewErr(serr.TErrNotfound, id)
		}

		from := s.getMap(s.status[id])
		if from == nil {
			return serr.NewErr(serr.TErrInval, s.status[id])
		}

		if (*to)[id] && s.status[id] != req.To {
			db.DFatalf("Task %v found in both %v and %v", id, s.status[id], req.To)
		}
	}

	for _, id := range req.Ids {
		if s.status[id] == req.To {
			continue
		}

		(*to)[id] = true
		from := s.getMap(s.status[id])
		delete(*from, id)
		s.status[id] = req.To
	}

	if req.To == proto.TaskStatus_TODO {
		s.todoCond.Broadcast()
	}

	return nil
}

func (s *TaskSrv) MoveTasksByStatus(ctx fs.CtxI, req proto.MoveTasksByStatusReq, rep *proto.MoveTasksByStatusRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	from := s.getMap(req.From)
	to := s.getMap(req.To)

	if from == nil {
		return serr.NewErr(serr.TErrInval, req.From)
	}

	if to == nil {
		return serr.NewErr(serr.TErrInval, req.To)
	}

	for id := range *from {
		if (*to)[id] {
			db.DFatalf("Task %v already in %v", id, req.To)
		}

		(*to)[id] = true
		s.status[id] = req.To
	}

	*from = make(map[int32]bool)

	if req.To == proto.TaskStatus_TODO {
		s.todoCond.Broadcast()
	}

	return nil
}

func (s *TaskSrv) GetTaskOutputs(ctx fs.CtxI, req proto.GetTaskOutputsReq, rep *proto.GetTaskOutputsRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

  for ix, id := range req.Ids {
		output := req.Outputs[ix]
		s.output[id] = output
	}

	return nil
}

func (s *TaskSrv) AcquireTasks(ctx fs.CtxI, req proto.AcquireTasksReq, rep *proto.AcquireTasksRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rep.Ids = s.get(proto.TaskStatus_TODO)
	for req.Wait && len(rep.Ids) == 0 {
		s.todoCond.Wait()
		rep.Ids = s.get(proto.TaskStatus_TODO)
	}

	for _, id := range rep.Ids {
		if s.wip[id] {
			db.DFatalf("Task %v already WIP", id)
		}
	}

	s.todo = make(map[int32]bool)
	for _, id := range rep.Ids {
		s.wip[id] = true
		s.status[id] = proto.TaskStatus_WIP
	}

	return nil
}

func (s *TaskSrv) GetTaskStats(ctx fs.CtxI, req proto.GetTaskStatsReq, rep *proto.GetTaskStatsRep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rep.Stats = &proto.TaskStats{
		NumTodo: int32(len(s.todo)),
		NumWip: int32(len(s.wip)),
		NumDone: int32(len(s.done)),
		NumError: int32(len(s.errored)),
	}

	return nil
}
