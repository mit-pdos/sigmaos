// The mr package implements a MapReduce library using sigmaos procs.
package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	"sigmaos/util/rand"
)

const (
	NCOORD               = 1
	RESTART              = "restart" // restart message from reducer
	MALICIOUS_MAPPER_BIN = "mr-m-malicious"
)

// mr_test puts pathnames of input files (split into bins) in
// MR/<job>/m/ and creates an fttask task for each one of them.  It
// also creates a number of reducer tasks (one for each reducer).
//
// The coordinator claims tasks and start procs for them, which
// process the claimed task.  Mapper i creates <r> output shards, one
// for each reducer and returns a bin of pathnames for the shards to
// the coordinator.
//
// If a mapper or reducer proc successfully exits, the coordinator
// marks the task as done and stores the pathnames returned by the
// mapper with the task.  If it fails, the coordinator will make the
// task runnable again and start new mapper/reducer procs to process
// the task.  If the coordinator fails, another coordinator will take
// over and claim tasks.

type TreduceTask struct {
	Task string `json:"Task"`
	Input Bin
}

type Coord struct {
	*sigmaclnt.SigmaClnt
	mftid           task.FtTaskSrvId
	rftid           task.FtTaskSrvId
	mftclnt         fttask_clnt.FtTaskClnt[Bin, Bin]
	rftclnt         fttask_clnt.FtTaskClnt[TreduceTask, Bin]
	jobRoot         string
	job             string
	nmaptask        int
	nreducetask     int
	maliciousMapper uint64
	linesz          string
	wordsz          string
	mapperbin       string
	reducerbin      string
	leaderclnt      *leaderclnt.LeaderClnt
	outdir          string
	intOutdir       string
	done            int32
	memPerTask      proc.Tmem
	stat            Stat
	perf            *perf.Perf
}

type Stat struct {
	Ntask          int
	Nmap           int
	Nreduce        int
	Nfail          int
	Nrestart       int
	NrecoverMap    int
	NrecoverReduce int
}

func (s *Stat) String() string {
	return fmt.Sprintf("{nT %d nM %d nR %d nfail %d nrestart %d nrecoverM %d nrecoverR %d}", s.Ntask, s.Nmap, s.Nreduce, s.Nfail, s.Nrestart, s.NrecoverMap, s.NrecoverReduce)
}

type NewProc func(fttask_clnt.Task[[]byte]) (*proc.Proc, error)

func NewCoord(args []string) (*Coord, error) {
	if len(args) != 12 {
		return nil, errors.New("NewCoord: wrong number of arguments")
	}
	c := &Coord{}
	c.jobRoot = args[1]
	c.job = args[0]
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	perf, _ := perf.NewPerf(proc.GetProcEnv(), perf.MRCOORD)
	c.perf = perf
	db.DPrintf(db.MR_COORD, "Made fslib job %v", c.job)
	c.SigmaClnt = sc
	m, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nmaptask %v isn't int", args[2])
	}
	n, err := strconv.Atoi(args[3])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nreducetask %v isn't int", args[3])
	}
	c.nmaptask = m
	c.nreducetask = n

	c.mapperbin = args[4]
	c.reducerbin = args[5]

	malmap, err := strconv.Atoi(args[9])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: maliciousMapper %v isn't int", args[9])
	}
	c.maliciousMapper = uint64(malmap)

	c.linesz = args[6]
	c.wordsz = args[7]

	mem, err := strconv.Atoi(args[8])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nreducetask %v isn't int", args[3])
	}
	c.memPerTask = proc.Tmem(mem)

	b, err := c.GetFile(JobOutLink(c.jobRoot, c.job))
	if err != nil {
		db.DFatalf("Error GetFile JobOutLink [%v]: %v", JobOutLink(c.jobRoot, c.job), err)
	}
	c.outdir = string(b)

	b, err = c.GetFile(JobIntOutLink(c.jobRoot, c.job))
	if err != nil {
		db.DFatalf("Error GetFile JobIntOutLink: %v", err)
	}
	c.intOutdir = string(b)

	c.Started()

	c.leaderclnt, err = leaderclnt.NewLeaderClnt(c.FsLib, LeaderElectDir(c.job)+"/coord-leader", 0)
	if err != nil {
		return nil, fmt.Errorf("NewCoord: NewLeaderclnt err %v", err)
	}

	c.mftid = task.FtTaskSrvId(args[10])
	c.rftid = task.FtTaskSrvId(args[11])

	return c, nil
}

func (c *Coord) newTask(bin string, args []string, mb proc.Tmem) *proc.Proc {
	pid := sp.GenPid(bin + "-" + c.job)
	p := proc.NewProcPid(pid, bin, args)
	//	if mb > 0 {
	//		p.AppendEnv("GOMEMLIMIT", strconv.Itoa(int(mb)*1024*1024))
	//	}
	p.SetMem(mb)
	return p
}

func (c *Coord) mapperProc(t fttask_clnt.Task[[]byte]) (*proc.Proc, error) {
	bin, err := fttask_clnt.Decode[Bin](t.Data)
	if err != nil {
		db.DFatalf("mapperProc: failed to convert data to bin %v %v", t.Data, err)
	}

	mapperbin := c.mapperbin
	// If running with malicious mappers, roll the dice and see if we should
	// spawn a benign mapper or a malicious one.
	if c.maliciousMapper > 0 {
		roll := rand.Int64(1000)
		if roll < c.maliciousMapper {
			// Roll successful: switch to malicious mapper
			mapperbin = MALICIOUS_MAPPER_BIN
		}
	}
	c.stat.Nmap += 1

	b, err := json.Marshal(bin)
	if err != nil {
		db.DFatalf("mapperProc: %v err %v", bin, err)
	}
	proc := c.newTask(mapperbin, []string{c.jobRoot, c.job, strconv.Itoa(c.nreducetask), string(b), c.intOutdir, c.linesz, c.wordsz}, c.memPerTask)
	return proc, nil
}

func (c *Coord) reducerProc(t fttask_clnt.Task[[]byte]) (*proc.Proc, error) {
	data, err := fttask_clnt.Decode[TreduceTask](t.Data)
	if err != nil {
		db.DFatalf("reducerProc: failed to convert data to task %v %v", t.Data, err)
	}

	outlink := ReduceOut(c.jobRoot, c.job) + data.Task
	outTarget := ReduceOutTarget(c.outdir, c.job) + data.Task
	c.stat.Nreduce += 1
	return c.newTask(c.reducerbin, []string{strconv.Itoa(int(t.Id)), string(c.rftclnt.ServerId()), outlink, outTarget, strconv.Itoa(c.nmaptask)}, c.memPerTask), nil
}

type Tresult struct {
	t   fttask_clnt.TaskId
	ok  bool
	ms  int64
	msg string
	res *Result
}

func (c *Coord) waitForTask(ftclnt fttask_clnt.FtTaskClnt[[]byte, []byte], start time.Time, ch chan Tresult, p *proc.Proc, t fttask_clnt.TaskId) {
	// Wait for the task to exit.
	status, err := c.WaitExit(p.GetPid())
	// Record end time.
	ms := time.Since(start).Milliseconds()
	if err == nil && status.IsStatusOK() {
		if c.maliciousMapper > 0 && p.GetProgram() == MALICIOUS_MAPPER_BIN {
			// If running with malicious mapper, then exit status should not be OK.
			// The task should be restarted automatically by the MR FT
			// infrastructure.  If the exit status *was* OK, then the output files
			// won't match, because the malicious mapper doesn't actually do the map
			// (it just touches some buckets it shouldn't have access to). Because of
			// this, letting the coordinator proceed by marking the task as done
			// should cause the test to fail.
			db.DPrintf(db.ERROR, "!!! WARNING: MALICIOUS MAPPER SUCCEEDED !!!")
		}
		r, err := NewResult(status.Data())
		if err != nil {
			db.DFatalf("NewResult %v err %v", status.Data(), err)
		}
		// mark task as done
		start := time.Now()
		encoded, err := fttask_clnt.Encode(r.OutBin)
		if err != nil {
			db.DFatalf("Encode %v err %v", r.OutBin, err)
		}
		if err := ftclnt.AddTaskOutputs([]fttask_clnt.TaskId{t}, [][]byte{encoded}, true); err != nil {
			db.DFatalf("MarkDone %v done err %v", t, err)
		}
		db.DPrintf(db.MR_COORD, "MarkDone latency: lat %v inner %v task %s", time.Since(start), r.MsInner, r.Task)
		r.MsOuter = ms
		ch <- Tresult{t, true, ms, status.Msg(), r}
	} else { // task failed; make it runnable again
		c.stat.Nfail += 1
		db.DPrintf(db.MR_COORD, "Task failed %v status %v", t, status)
		if status != nil && status.Msg() == RESTART {
			// reducer indicates to run some mappers again
			s := newStringSlice(status.Data().([]interface{}))
			c.restart(s, t)
		} else { // if failure but not restart, rerun task immediately again
			if err := ftclnt.MoveTasks([]fttask_clnt.TaskId{t}, fttask_clnt.TODO); err != nil {
				db.DFatalf("MarkRunnable %v err %v", t, err)
			}
		}
		ch <- Tresult{t, false, ms, "", nil}
	}
}

func (c *Coord) runTasks(ftclnt fttask_clnt.FtTaskClnt[[]byte, []byte], ch chan Tresult, ids []fttask_clnt.TaskId, f NewProc) {
	db.DPrintf(db.MR_COORD, "runTasks %v", len(ids))
	start := time.Now()
	tasks, err := ftclnt.ReadTasks(ids)
	if err != nil {
		db.DFatalf("ReadTasks %v err %v", ids, err)
	}
	db.DPrintf(db.MR_COORD, "runTasks: read tasks %v time: %v", len(tasks), time.Since(start))

	// create all proc objects first so we can spawn them all in
	// quick succession to try to balance load across machines
	procs := make([]*proc.Proc, len(tasks))
	for i, t := range tasks {
		proc, err := f(t)
		if err != nil {
			db.DFatalf("Err spawn task: %v", err)
		}

		procs[i] = proc
	}

	for i, t := range tasks {
		proc := procs[i]
		db.DPrintf(db.MR_COORD, "prep to spawn proc %v", proc.GetPid())
		start := time.Now()
		err = c.Spawn(proc)
		if err != nil {
			db.DFatalf("Err spawn task: %v", err)
		}
		go c.waitForTask(ftclnt, start, ch, proc, t.Id)
	}
}

func newStringSlice(data []interface{}) []string {
	s := make([]string, 0, len(data))
	for _, o := range data {
		s = append(s, o.(string))
	}
	return s
}

func (c *Coord) startTasks(ft fttask_clnt.FtTaskClnt[[]byte, []byte], ch chan Tresult, f NewProc) int {
	start := time.Now()
	tns, _, err := ft.AcquireTasks(false)
	if err != nil {
		db.DFatalf("startTasks err %v\n", err)
	}
	db.DPrintf(db.MR_COORD, "startTasks %v time: %v", len(tns), time.Since(start))
	c.runTasks(ft, ch, tns, f)
	return len(tns)
}

// A reducer failed because it couldn't read its input file; we must
// restart mapper.  We let all mappers and reducers finish, before
// starting a new round for the mappers and reducers that must be
// restarted, which avoids restarting a mapper several times (because
// several reducers may ask the mapper to be restarted).
func (c *Coord) restart(files []string, task fttask_clnt.TaskId) {
	db.DPrintf(db.ALWAYS, "restart: files %v for %v\n", files, task)
	for _, f := range files {
		if err := c.Remove(f); err != nil {
			db.DPrintf(db.ALWAYS, "remove %v err %v\n", f, err)
		}
	}
	// Record that we have to rerun reducer task
	if err := c.rftclnt.MoveTasks([]fttask_clnt.TaskId{task}, fttask_clnt.ERROR); err != nil {
		db.DPrintf(db.ALWAYS, "restart reducer %v err %v\n", task, err)
	}
}

// Mark all error-ed tasks as runnable
func (c *Coord) doRestart() bool {
	m, err := c.rftclnt.MoveTasksByStatus(fttask_clnt.ERROR, fttask_clnt.TODO)
	if err != nil {
		db.DFatalf("Restart reducers err %v\n", err)
	}
	if m > 0 {
		// if a reducer couldn't read its input files, mark all
		// mappers as failed so that they will be restarted.
		_, err := c.mftclnt.MoveTasksByStatus(fttask_clnt.DONE, fttask_clnt.ERROR)
		if err != nil {
			db.DFatalf("MarkDoneError err %v\n", err)
		}
	}
	n, err := c.mftclnt.MoveTasksByStatus(fttask_clnt.ERROR, fttask_clnt.TODO)
	if err != nil {
		db.DFatalf("Restart mappers err %v\n", err)
	}
	if n+m > 0 {
		db.DPrintf(db.ALWAYS, "doRestart(): restart %d tasks\n", n+m)
	}
	c.stat.Nrestart += int(n + m)
	return n+m > 0
}

func (c *Coord) makeReduceBins() error {
	mns, err := c.mftclnt.GetTasksByStatus(fttask_clnt.DONE)
	if err != nil {
		return err
	}
	rnsTodo, err := c.rftclnt.GetTasksByStatus(fttask_clnt.TODO)
	if err != nil {
		return err
	}

	rnsDone, err := c.rftclnt.GetTasksByStatus(fttask_clnt.DONE)
	if err != nil {
		return err
	}

	// get all reducers (including those that succeeded in previous rounds) in sorted order to ensure
	// any restarted reducers are given the exact same files as before
	rns := append(rnsDone, rnsTodo...)
	sort.Slice(rns, func(i, j int) bool {
		return rns[i] < rns[j]
	})

	reduceBinIn := make(map[fttask_clnt.TaskId]Bin, c.nreducetask)

	for _, n := range rns {
		reduceBinIn[n] = make(Bin, c.nmaptask)
	}

	obins, err := c.mftclnt.GetTaskOutputs(mns)
	if err != nil {
		return err
	}

	for j, obin := range obins {
		for i, s := range obin {
			reduceBinIn[rns[i]][j] = s
		}
	}

	// db.DPrintf(db.MR_COORD, "makeReduceBins: reduceBinIn %v", reduceBinIn)

	start := time.Now()
	rtaskData, err := c.rftclnt.ReadTasks(rnsTodo)
	if err != nil {
		db.DPrintf(db.MR_COORD, "ReadTasks %v err %v", rns, err)
		return err
	}
	db.DPrintf(db.MR_COORD, "makeReduceBins: read %v tasks %v", len(rtaskData), time.Since(start))

	for i, t := range rtaskData {
		if reduceBin, ok := reduceBinIn[t.Id]; ok {
			rtaskData[i].Data.Input = reduceBin
		} else {
			db.DPrintf(db.MR_COORD, "makeReduceBins: no input for %v", t.Id)
			continue
		}
	}

	// if we have a lot of mappers, this can be a lot of data, whihc exceeds the 2MB limit per gRPC
	// message and/or the 1MB limit for etcd, so we break it up into batches and use 900 KB messages
	// to be safe
	const (
		maxBatchSize = 900 * 1024      // 900 KB
		taskOverhead = 64              // Proto overhead estimate
	)

	batched := make([]*fttask_clnt.Task[TreduceTask], 0, len(rtaskData))
	currentBatchSize := 0

	start = time.Now()
	for i := range rtaskData {
		task := &rtaskData[i]
		b, err := json.Marshal(task)
		if err != nil {
			db.DFatalf("json.Marshal %v err %v", task, err)
		}
		estSize := len(b) + taskOverhead

		if currentBatchSize + estSize > maxBatchSize && len(batched) > 0 {
			if _, err = c.rftclnt.EditTasks(batched); err != nil {
				db.DPrintf(db.MR_COORD, "EditTasks batch err %v", err)
			}
			db.DPrintf(db.MR_COORD, "makeReduceBins: EditTasks %v tasks %v", len(batched), time.Since(start))
			start = time.Now()
			batched = nil
			currentBatchSize = 0
		}

		batched = append(batched, task)
		currentBatchSize += estSize
	}

	// Send remaining tasks
	if len(batched) > 0 {
		if _, err = c.rftclnt.EditTasks(batched); err != nil {
			db.DPrintf(db.MR_COORD, "EditTasks final batch err %v", err)
		}
		db.DPrintf(db.MR_COORD, "makeReduceBins: EditTasks %v tasks %v", len(batched), time.Since(start))
	}

	return nil
}

// XXX do something for stragglers?
func (c *Coord) Round(ttype string) {
	mapsDone := false
	start := time.Now()
	ch := make(chan Tresult)
	for m := 0; ; m-- {
		if ttype == "map" {
			m += c.startTasks(c.mftclnt.AsRawClnt(), ch, c.mapperProc)
		} else if ttype == "reduce" {
			m += c.startTasks(c.rftclnt.AsRawClnt(), ch, c.reducerProc)
		} else if ttype == "all" {
			m += c.startTasks(c.mftclnt.AsRawClnt(), ch, c.mapperProc)
			db.DPrintf(db.MR_COORD, "startTasks mappers %v", m)
			m += c.startTasks(c.rftclnt.AsRawClnt(), ch, c.reducerProc)
			db.DPrintf(db.MR_COORD, "startTasks add reducers %v", m)
		} else {
			db.DFatalf("Unknown ttype: %v", ttype)
		}
		if m <= 0 {
			break
		}
		res := <-ch
		db.DPrintf(db.MR_COORD, "Round: task done %v ok %v msg %v", res.t, res.ok, res.msg)
		if res.res != nil {
			db.DPrintf(db.MR_COORD, "Round: task %v res: msInner %d msOuter %d res %v\n", res.t, res.res.MsInner, res.res.MsOuter, res.res)
		}
		if res.ok {
			if err := c.AppendFileJson(MRstats(c.jobRoot, c.job), res.res); err != nil {
				db.DFatalf("Appendfile %v err %v\n", MRstats(c.jobRoot, c.job), err)
			}
			db.DPrintf(db.ALWAYS, "tasks left %d/%d\n", m-1, c.nmaptask+c.nreducetask)
			if !mapsDone && m < c.nmaptask {
				mapsDone = true
				db.DPrintf(db.ALWAYS, "Mapping took %vs\n", time.Since(start).Seconds())
			}
		}
	}
}

func (c *Coord) Work() {
	db.DPrintf(db.MR_COORD, "Try acquire leadership coord %v job %v", c.ProcEnv().GetPID(), c.job)

	// Try to become the leading coordinator.
	if err := c.leaderclnt.LeadAndFence(nil, []string{JobDir(c.jobRoot, c.job)}); err != nil {
		db.DFatalf("LeadAndFence err %v", err)
	}

	db.DPrintf(db.ALWAYS, "leader %s nmap %v nreduce %v\n", c.job, c.nmaptask, c.nreducetask)

	var err error
	c.mftclnt = fttask_clnt.NewFtTaskClnt[Bin, Bin](c.FsLib, c.mftid)
	c.rftclnt = fttask_clnt.NewFtTaskClnt[TreduceTask, Bin](c.FsLib, c.rftid)

	crash.Failer(c.FsLib, crash.MRCOORD_CRASH, func(e crash.Tevent) {
		crash.CrashMsg(c.stat.String())
	})
	crash.Failer(c.FsLib, crash.MRCOORD_PARTITION, func(e crash.Tevent) {
		crash.PartitionNamed(c.FsLib)
	})

	start := time.Now()
	if n, err := c.mftclnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO); err != nil {
		db.DFatalf("RecoverTasks mapper err %v", err)
	} else {
		c.stat.NrecoverMap = int(n)
		db.DPrintf(db.MR_COORD, "Recover %d map tasks took %v", n, time.Since(start))
	}

	start = time.Now()
	if n, err := c.rftclnt.MoveTasksByStatus(fttask_clnt.WIP, fttask_clnt.TODO); err != nil {
		db.DFatalf("RecoverTasks reducer err %v", err)
	} else {
		c.stat.NrecoverReduce = int(n)
		db.DPrintf(db.MR_COORD, "Recover %d reduce tasks took %v", n, time.Since(start))
	}

	mftStats, err := c.mftclnt.Stats()
	if err != nil {
		db.DFatalf("Stats err %v\n", err)
	}
	rftStats, err := c.rftclnt.Stats()
	if err != nil {
		db.DFatalf("Stats err %v\n", err)
	}
	c.stat.Ntask = int(mftStats.NumDone + rftStats.NumDone + mftStats.NumError + rftStats.NumError + mftStats.NumTodo + rftStats.NumTodo + mftStats.NumError + rftStats.NumError)

	start = time.Now()
	c.doRestart()
	db.DPrintf(db.MR_COORD, "doRestart took %v", time.Since(start))
	jobStart := time.Now()

	for {
		//		c.Round("all")
		start := time.Now()
		c.Round("map")
		m, err := c.mftclnt.GetNTasks(fttask_clnt.DONE)
		db.DPrintf(db.ALWAYS, "run round %d", m)
		if err != nil {
			db.DFatalf("NtaskDone err %v\n", err)
		}
		if int(m) == c.nmaptask {
			ms := time.Since(start).Milliseconds()
			db.DPrintf(db.ALWAYS, "map phase took %vms\n", ms)

			err := c.makeReduceBins()
			if err != nil {
				db.DFatalf("ReduceBins err %v", err)
			}
			c.Round("reduce")
		}
		if !c.doRestart() {
			break
		}
	}

	// double check we are done
	n, err := c.mftclnt.GetNTasks(fttask_clnt.DONE)
	if err != nil {
		db.DFatalf("NtaskDone mappers err %v\n", err)
	}
	m, err := c.rftclnt.GetNTasks(fttask_clnt.DONE)
	if err != nil {
		db.DFatalf("NtaskDone reducers err %v\n", err)
	}
	if int(n+m) < c.nmaptask+c.nreducetask {
		db.DFatalf("job isn't done %v+%v != %v+%v", n, m, c.nmaptask, c.nreducetask)
	}

	db.DPrintf(db.ALWAYS, "job done stat %v", &c.stat)

	atomic.StoreInt32(&c.done, 1)

	db.DPrintf(db.ALWAYS, "E2e bench took %v", time.Since(jobStart))
	JobDone(c.FsLib, c.jobRoot, c.job)

	defer c.perf.Done()

	c.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK", c.stat))
}
