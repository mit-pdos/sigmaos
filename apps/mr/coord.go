// The mr package implements a MapReduce library using sigmaos procs.
package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/ft/task"
	ftclnt "sigmaos/ft/task/clnt"
	"sigmaos/ft/task/fttaskmgr"
	ftmgr "sigmaos/ft/task/fttaskmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	"sigmaos/util/rand"
	"sigmaos/util/spstats"
)

const (
	NCOORD               = 1
	RESTART              = "restart" // restart message from reducer
	MALICIOUS_MAPPER_BIN = "mr-m-malicious"

	// SpecCheckInterval is how often the coordinator looks for stragglers to
	// speculatively back up, when specEnabled.
	SpecCheckInterval = 500 * time.Millisecond
	// SpecMinProgress is the fraction of a phase's tasks that must already be
	// DONE before any backups are allowed to fire -- the classic MapReduce
	// heuristic of only speculating once a phase is mostly finished.
	SpecMinProgress = 0.75
	// SpecSlowFactor: a still-running task is a straggler once it has run
	// longer than this factor times the average completion time of tasks
	// that already finished in the same phase. Kept high (rather than the
	// ~1.5x textbook value) because task durations here have enough natural
	// variance (S3 read latency, local machine contention) that a low
	// factor triggers on ordinary slow tasks instead of genuine stragglers.
	SpecSlowFactor = 1.5
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
	Task  string `json:"Task"`
	Input Bin
}

type Coord struct {
	*sigmaclnt.SigmaClnt
	mftid           task.FtTaskSvcId
	rftid           task.FtTaskSvcId
	mftclnt         ftclnt.FtTaskClnt[Bin, Bin]
	rftclnt         ftclnt.FtTaskClnt[TreduceTask, Bin]
	mcoord          *fttaskmgr.FtTaskCoord[[]byte, []byte]
	rcoord          *fttaskmgr.FtTaskCoord[[]byte, []byte]
	jobRoot         string
	job             string
	nmaptask        int
	nreducetask     int
	maliciousMapper uint64
	slowTaskId      int64
	slowdownMs      int
	linesz          string
	wordsz          string
	mapperbin       string
	reducerbin      string
	leaderclnt      *leaderclnt.LeaderClnt
	outdir          string
	intOutdir       string
	memPerTask      proc.Tmem
	stat            AStat
	perf            *perf.Perf

	// Classic speculative execution: when specEnabled, the coordinator
	// launches a backup execution of any map/reduce task that is running
	// much slower than its peers, once most of the phase is done. Whichever
	// attempt (original or backup) finishes first wins; the other is
	// evicted. specMu guards every field below it.
	specEnabled bool
	specDone    chan struct{}
	specMu      sync.Mutex
	mStart      map[ftclnt.TaskId]time.Time
	rStart      map[ftclnt.TaskId]time.Time
	mBackedUp   map[ftclnt.TaskId]bool
	rBackedUp   map[ftclnt.TaskId]bool
	mAttempts   map[ftclnt.TaskId][]sp.Tpid
	rAttempts   map[ftclnt.TaskId][]sp.Tpid
	mDurations  []time.Duration
	rDurations  []time.Duration
}

type AStat struct {
	Ntask          spstats.Tcounter
	Nmap           spstats.Tcounter
	Nreduce        spstats.Tcounter
	Nfail          spstats.Tcounter
	Nrestart       spstats.Tcounter
	NrecoverMap    spstats.Tcounter
	NrecoverReduce spstats.Tcounter
	Nspeculate     spstats.Tcounter
}

func (s *AStat) String() string {
	return fmt.Sprintf("{nT %d nM %d nR %d nfail %d nrestart %d nrecoverM %d nrecoverR %d nspec %d}", s.Ntask.Load(), s.Nmap.Load(), s.Nreduce.Load(), s.Nfail.Load(), s.Nrestart.Load(), s.NrecoverMap.Load(), s.NrecoverReduce.Load(), s.Nspeculate.Load())
}

type NewProc func(ftclnt.Task[[]byte]) (*proc.Proc, error)

func NewCoord(args []string) (*Coord, error) {
	if len(args) != 15 {
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

	c.mftid = task.FtTaskSvcId(args[10])
	c.rftid = task.FtTaskSvcId(args[11])

	// Parse straggler-injection args (-1 slowTaskId disables it).
	slowTaskId, err := strconv.ParseInt(args[12], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("NewCoord: slowTaskId %v isn't int64", args[12])
	}
	c.slowTaskId = slowTaskId

	slowdownMs, err := strconv.Atoi(args[13])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: slowdownMs %v isn't int", args[13])
	}
	c.slowdownMs = slowdownMs

	// Parse the speculative-execution toggle and initialize its bookkeeping.
	specEnabled, err := strconv.ParseBool(args[14])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: specEnabled %v isn't bool", args[14])
	}
	c.specEnabled = specEnabled
	c.specDone = make(chan struct{})
	c.mStart = make(map[ftclnt.TaskId]time.Time)
	c.rStart = make(map[ftclnt.TaskId]time.Time)
	c.mBackedUp = make(map[ftclnt.TaskId]bool)
	c.rBackedUp = make(map[ftclnt.TaskId]bool)
	c.mAttempts = make(map[ftclnt.TaskId][]sp.Tpid)
	c.rAttempts = make(map[ftclnt.TaskId][]sp.Tpid)

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

// mapperProc builds (but doesn't spawn) a mapper proc for task t; called both
// for a task's primary attempt (via fttaskmgr) and for a speculative backup
// (via runBackupMap).
func (c *Coord) mapperProc(t ftclnt.Task[[]byte]) (*proc.Proc, error) {
	return c.buildMapperProc(t, false)
}

// buildMapperProc builds (but doesn't spawn) a mapper proc for task t.
// isBackup is true when building a speculative backup: the straggler-
// injection delay (when t is the designated straggler task) is only applied
// to the primary attempt. Applying it to a backup too would force it to pay
// the identical fixed delay as the attempt it's racing against, so it could
// never win.
func (c *Coord) buildMapperProc(t ftclnt.Task[[]byte], isBackup bool) (*proc.Proc, error) {
	bin, err := ftclnt.Decode[Bin](t.Data)
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
	c.stat.Nmap.Add(1)

	b, err := json.Marshal(bin)
	if err != nil {
		db.DFatalf("mapperProc: %v err %v", bin, err)
	}
	// Delay only the primary attempt at the one task designated as the
	// straggler; every other task/attempt gets "0" (no delay).
	slowdownMs := 0
	if !isBackup && int64(t.Id) == c.slowTaskId {
		slowdownMs = c.slowdownMs
	}
	proc := c.newTask(mapperbin, []string{c.jobRoot, c.job, strconv.Itoa(c.nreducetask), string(b), c.intOutdir, c.linesz, c.wordsz, strconv.Itoa(slowdownMs)}, c.memPerTask)
	c.recordAttempt(t.Id, proc.GetPid(), true)
	return proc, nil
}

// reducerProc is mapperProc's mirror for the reduce phase.
func (c *Coord) reducerProc(t ftclnt.Task[[]byte]) (*proc.Proc, error) {
	data, err := ftclnt.Decode[TreduceTask](t.Data)
	if err != nil {
		db.DFatalf("reducerProc: failed to convert data to task %v %v", t.Data, err)
	}
	outlink := ReduceOut(c.jobRoot, c.job) + data.Task
	outTarget := ReduceOutTarget(c.outdir, c.job) + data.Task
	c.stat.Nreduce.Add(1)
	p := c.newTask(c.reducerbin, []string{strconv.Itoa(int(t.Id)), string(c.rftclnt.ServiceId()), outlink, outTarget, strconv.Itoa(c.nmaptask)}, c.memPerTask)
	c.recordAttempt(t.Id, p.GetPid(), false)
	return p, nil
}

// recordAttempt notes that pid is (one of) the running attempt(s) for task
// id -- used later to evict the loser once a winning result is known, and
// (for the very first attempt) as the start time stragglers are measured
// against.
func (c *Coord) recordAttempt(id ftclnt.TaskId, pid sp.Tpid, isMap bool) {
	c.specMu.Lock()
	defer c.specMu.Unlock()
	if isMap {
		if _, ok := c.mStart[id]; !ok {
			c.mStart[id] = time.Now()
		}
		c.mAttempts[id] = append(c.mAttempts[id], pid)
	} else {
		if _, ok := c.rStart[id]; !ok {
			c.rStart[id] = time.Now()
		}
		c.rAttempts[id] = append(c.rAttempts[id], pid)
	}
}

// speculate periodically checks for straggling map/reduce tasks and
// launches a backup execution for each one, implementing classic MapReduce
// speculative execution. It runs for the lifetime of the job and returns
// once c.specDone is closed.
func (c *Coord) speculate(ch chan<- ftmgr.Tresult[[]byte, []byte]) {
	ticker := time.NewTicker(SpecCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.specDone:
			return
		case <-ticker.C:
			c.speculateMap(ch)
			c.speculateReduce(ch)
		}
	}
}

// speculateMap looks for map tasks running much slower than the average of
// already-completed map tasks, once most of the map phase is done, and
// launches one backup execution for each (at most once per task).
func (c *Coord) speculateMap(ch chan<- ftmgr.Tresult[[]byte, []byte]) {
	// Get number of tasks if enough of the map phase has completed to consider speculation. If not, return early.
	done, err := c.mftclnt.GetNTasks(ftclnt.DONE)
	if err != nil || float64(done) < SpecMinProgress*float64(c.nmaptask) {
		return
	}

	// Get the list of currently running map tasks (WIP). If none, return early.
	wip, err := c.mftclnt.GetTasksByStatus(ftclnt.WIP)
	if err != nil || len(wip) == 0 {
		return
	}

	// Compute the average duration of completed map tasks. If none have completed, return early.
	avg := c.avgDuration(true)
	if avg <= 0 {
		return
	}

	// Check if each running map task has exceeded the threshold duration and claim a backup if so.
	threshold := time.Duration(float64(avg) * SpecSlowFactor)
	now := time.Now()
	for _, id := range wip {
		if c.claimMapBackup(id, now, threshold) {
			go c.runBackupMap(id, ch)
		}
	}
}

// speculateReduce is speculateMap's mirror for the reduce phase.
func (c *Coord) speculateReduce(ch chan<- ftmgr.Tresult[[]byte, []byte]) {
	// Get number of tasks if enough of the reduce phase has completed to consider speculation. If not, return early.
	done, err := c.rftclnt.GetNTasks(ftclnt.DONE)
	if err != nil || float64(done) < SpecMinProgress*float64(c.nreducetask) {
		return
	}

	// Get the list of currently running reduce tasks (WIP). If none, return early.
	wip, err := c.rftclnt.GetTasksByStatus(ftclnt.WIP)
	if err != nil || len(wip) == 0 {
		return
	}

	// Compute the average duration of completed reduce tasks. If none have completed, return early.
	avg := c.avgDuration(false)
	if avg <= 0 {
		return
	}

	// Check if each running reduce task has exceeded the threshold duration and claim a backup if so.
	threshold := time.Duration(float64(avg) * SpecSlowFactor)
	now := time.Now()
	for _, id := range wip {
		if c.claimReduceBackup(id, now, threshold) {
			go c.runBackupReduce(id, ch)
		}
	}
}

// claimMapBackup reports whether map task id has been running longer than
// threshold and doesn't already have a backup in flight, atomically marking
// it as backed-up if so (so at most one backup is ever launched per task).
func (c *Coord) claimMapBackup(id ftclnt.TaskId, now time.Time, threshold time.Duration) bool {
	c.specMu.Lock()
	defer c.specMu.Unlock()

	start, ok := c.mStart[id]
	if !ok || c.mBackedUp[id] || now.Sub(start) <= threshold {
		return false
	}
	c.mBackedUp[id] = true
	return true
}

// claimReduceBackup is claimMapBackup's mirror for the reduce phase.
func (c *Coord) claimReduceBackup(id ftclnt.TaskId, now time.Time, threshold time.Duration) bool {
	c.specMu.Lock()
	defer c.specMu.Unlock()

	start, ok := c.rStart[id]
	if !ok || c.rBackedUp[id] || now.Sub(start) <= threshold {
		return false
	}
	c.rBackedUp[id] = true
	return true
}

// avgDuration returns the average completion time of map (isMap) or reduce
// (!isMap) tasks that have finished so far in this job, or 0 if none have.
func (c *Coord) avgDuration(isMap bool) time.Duration {
	c.specMu.Lock()
	defer c.specMu.Unlock()

	durs := c.mDurations
	if !isMap {
		durs = c.rDurations
	}
	if len(durs) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range durs {
		sum += d
	}
	return sum / time.Duration(len(durs))
}

// runBackupMap spawns a second, backup execution of an already in-progress
// map task directly (bypassing the normal claim-from-TODO flow, since the
// task is still legitimately WIP under its original attempt), and reports
// its result on ch exactly like fttaskmgr's own runTask/waitForTask would.
func (c *Coord) runBackupMap(id ftclnt.TaskId, ch chan<- ftmgr.Tresult[[]byte, []byte]) {
	// Read the task information for the given map task ID from the raw client. If the task cannot be read, return early.
	raw := c.mftclnt.AsRawClnt()
	tasks, err := raw.ReadTasks([]ftclnt.TaskId{id})
	if err != nil || len(tasks) == 0 {
		return
	}

	// Construct the process for the map task using the mapperProc function. If this fails, return early.
	p, err := c.buildMapperProc(tasks[0], true)
	if err != nil {
		return
	}

	c.stat.Nspeculate.Add(1)
	db.DPrintf(db.ALWAYS, "speculate: backup mapper for task %v", id)
	start := time.Now()

	// Spawn the process for the backup map task and wait for it to start. If either step fails, return early.
	if err := c.Spawn(p); err != nil {
		return
	}
	if err := c.WaitStart(p.GetPid()); err != nil {
		return
	}
	// Wait for the backup map task to exit and collect its status and error.
	status, err := c.WaitExit(p.GetPid())
	if err != nil {
		return
	}
	// Report the result on the channel exactly like fttaskmgr's runTask/waitForTask would.
	ch <- ftmgr.Tresult[[]byte, []byte]{
		Ms:     time.Since(start),
		Err:    err,
		Status: status,
		Proc:   p,
		Id:     id,
		Ftclnt: raw,
	}
}

// runBackupReduce is runBackupMap's mirror for the reduce phase.
func (c *Coord) runBackupReduce(id ftclnt.TaskId, ch chan<- ftmgr.Tresult[[]byte, []byte]) {
	raw := c.rftclnt.AsRawClnt()
	tasks, err := raw.ReadTasks([]ftclnt.TaskId{id})
	if err != nil || len(tasks) == 0 {
		return
	}
	p, err := c.reducerProc(tasks[0])
	if err != nil {
		return
	}
	c.stat.Nspeculate.Add(1)
	db.DPrintf(db.ALWAYS, "speculate: backup reducer for task %v", id)
	start := time.Now()
	if err := c.Spawn(p); err != nil {
		return
	}

	// Wait for the backup reduce task to start. If this fails, return early.
	if err := c.WaitStart(p.GetPid()); err != nil {
		return
	}

	// Wait for the backup reduce task to exit and collect its status and error.
	status, err := c.WaitExit(p.GetPid())
	if err != nil {
		return
	}
	ch <- ftmgr.Tresult[[]byte, []byte]{
		Ms:     time.Since(start),
		Err:    err,
		Status: status,
		Proc:   p,
		Id:     id,
		Ftclnt: raw,
	}
}

// evictSiblings kills every other recorded attempt for task id and clears
// its speculation bookkeeping, so a stale attempt can't later report
// success once the task's fate (win, restart, or failure) is decided.
func (c *Coord) evictSiblings(id ftclnt.TaskId, exclude sp.Tpid, isMap bool) {
	c.specMu.Lock()
	var losers []sp.Tpid
	if isMap {
		for _, pid := range c.mAttempts[id] {
			if pid != exclude {
				losers = append(losers, pid)
			}
		}
		delete(c.mAttempts, id)
		delete(c.mStart, id)
		delete(c.mBackedUp, id)
	} else {
		for _, pid := range c.rAttempts[id] {
			if pid != exclude {
				losers = append(losers, pid)
			}
		}
		delete(c.rAttempts, id)
		delete(c.rStart, id)
		delete(c.rBackedUp, id)
	}
	c.specMu.Unlock()
	for _, pid := range losers {
		db.DPrintf(db.MR_COORD, "evictSiblings: evicting %v for task %v (exclude %v)", pid, id, exclude)
		if err := c.Evict(pid); err != nil {
			db.DPrintf(db.MR_COORD, "evictSiblings: Evict %v err %v", pid, err)
		}
	}
}

// taskWon records that pid won task id (recording its duration for the
// straggler-detection average), and evicts any other attempt (speculative
// backup or original) still running for the same task.
func (c *Coord) taskWon(id ftclnt.TaskId, winner sp.Tpid, dur time.Duration, isMap bool) {
	c.specMu.Lock()
	if isMap {
		c.mDurations = append(c.mDurations, dur)
	} else {
		c.rDurations = append(c.rDurations, dur)
	}
	c.specMu.Unlock()
	c.evictSiblings(id, winner, isMap)
}

// resetSpeculation clears map-side speculation bookkeeping (including
// mDurations, now stale) and evicts any recorded attempt, since all
// previously-done mappers are about to be redone (see doRestart). Reducers
// aren't bulk-restarted this way, so reduce-side state is left alone.
func (c *Coord) resetSpeculation() {
	c.specMu.Lock()
	var losers []sp.Tpid
	for _, pids := range c.mAttempts {
		losers = append(losers, pids...)
	}
	c.mStart = make(map[ftclnt.TaskId]time.Time)
	c.mBackedUp = make(map[ftclnt.TaskId]bool)
	c.mAttempts = make(map[ftclnt.TaskId][]sp.Tpid)
	c.mDurations = nil
	c.specMu.Unlock()
	for _, pid := range losers {
		db.DPrintf(db.MR_COORD, "resetSpeculation: evicting stale attempt %v", pid)
		if err := c.Evict(pid); err != nil {
			db.DPrintf(db.MR_COORD, "resetSpeculation: Evict %v err %v", pid, err)
		}
	}
}

func newStringSlice(data []interface{}) []string {
	s := make([]string, 0, len(data))
	for _, o := range data {
		s = append(s, o.(string))
	}
	return s
}

// A reducer failed because it couldn't read its input file; we must
// restart mapper.  We let all mappers and reducers finish, before
// restarting any mappers and reducers, which avoids restarting a
// mapper several times (because several reducers may ask the mapper
// to be restarted).
func (c *Coord) restart(files []string, task ftclnt.TaskId) {
	db.DPrintf(db.ALWAYS, "restart: files %v for %v\n", files, task)
	for _, f := range files {
		if err := c.Remove(f); err != nil {
			db.DPrintf(db.ALWAYS, "remove %v err %v\n", f, err)
		}
	}
	// Record that we have to rerun reducer task
	if err := c.rftclnt.MoveTasks([]ftclnt.TaskId{task}, ftclnt.ERROR); err != nil {
		db.DPrintf(db.ALWAYS, "restart reducer %v err %v\n", task, err)
	}
}

// Mark all errored mapper tasks as runnable. If there are errored
// reducers, mark all mappers as errored.
func (c *Coord) doRestart() {
	start := time.Now()
	ts, err := c.rftclnt.GetTasksByStatus(ftclnt.ERROR)
	if err != nil {
		db.DFatalf("doRestart: move error err %v\n", err)
	}
	if len(ts) > 0 {
		// if a reducer couldn't read its input files, mark all
		// mappers as failed so that they will be restarted.
		_, err := c.mftclnt.MoveTasksByStatus(ftclnt.DONE, ftclnt.ERROR)
		if err != nil {
			db.DFatalf("doRestart: move done err %v\n", err)
		}
	}
	n, err := c.mftclnt.MoveTasksByStatus(ftclnt.ERROR, ftclnt.TODO)
	if err != nil {
		db.DFatalf("doRestart:  mappers error err %v", err)
	}
	m := int32(len(ts))
	if n+m > 0 {
		db.DPrintf(db.ALWAYS, "doRestart: restart %d tasks", n+m)
	}
	spstats.Inc(&c.stat.Nrestart, int64(n+m))
	db.DPrintf(db.MR_COORD, "doRestart took %v", time.Since(start))
}

func (c *Coord) updateReducers(ids []ftclnt.TaskId, bins map[ftclnt.TaskId]Bin) error {
	start := time.Now()
	rtaskData, err := c.rftclnt.ReadTasks(ids)
	if err != nil {
		db.DPrintf(db.MR_COORD, "updateReducers: ReadTasks %v err %v", ids, err)
		return err
	}

	db.DPrintf(db.MR_COORD, "updateReducers: read %v tasks %v", len(rtaskData), time.Since(start))

	for i, t := range ids {
		if reduceBin, ok := bins[t]; ok {
			rtaskData[i].Data.Input = reduceBin
		} else {
			db.DFatalf("updateReducers: no input for %v", t)
		}
	}

	// XXX should the batch update be moved into ft/task/clnt?

	// if we have a lot of mappers and errored reducers, this can be a
	// lot of data, which exceeds the 2MB limit per gRPC message
	// and/or the 1MB limit for etcd, so we break it up into batches
	// and use 900 KB messages to be safe
	const (
		maxBatchSize = 900 * 1024 // 900 KB
		taskOverhead = 64         // Proto overhead estimate
	)

	batched := make([]*ftclnt.Task[TreduceTask], 0, len(rtaskData))
	currentBatchSize := 0

	start = time.Now()
	for i := range rtaskData {
		task := &rtaskData[i]
		b, err := json.Marshal(task)
		if err != nil {
			db.DFatalf("json.Marshal %v err %v", task, err)
		}
		estSize := len(b) + taskOverhead

		if currentBatchSize+estSize > maxBatchSize && len(batched) > 0 {
			if _, err = c.rftclnt.EditTasks(batched); err != nil {
				db.DPrintf(db.MR_COORD, "EditTasks batch err %v", err)
			}
			db.DPrintf(db.MR_COORD, "updateReducers: EditTasks %v tasks %v", len(batched), time.Since(start))
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
		db.DPrintf(db.MR_COORD, "updateReducers: EditTasks %v tasks %v", len(batched), time.Since(start))
	}

	// now all mappers are done and reduce tasks updated, resurrect
	// errored-out reducers, if any
	_, err = c.rftclnt.MoveTasksByStatus(ftclnt.ERROR, ftclnt.TODO)
	if err != nil {
		return err
	}

	return nil
}

func (c *Coord) createReducers(bins map[ftclnt.TaskId]Bin) error {
	tasks := make([]*ftclnt.Task[TreduceTask], c.nreducetask)
	for r := 0; r < c.nreducetask; r++ {
		t := TreduceTask{strconv.Itoa(r), bins[int32(r)]}
		tasks[r] = &ftclnt.Task[TreduceTask]{Id: ftclnt.TaskId(r), Data: t}
	}

	// XXX maybe this will be too big for a single gRPC message, and should
	// be batched like updateReducers?
	start := time.Now()
	if err := c.rftclnt.SubmitTasks(tasks); err != nil {
		return err
	}
	db.DPrintf(db.MR_COORD, "createReducers: %v took %v", c.nreducetask, time.Since(start))
	return nil
}

// Assumes no more mappers and reducers in WIP; that is, all mappers
// are done. Creates all reduce tasks or updates the ones that are
// errored out.
func (c *Coord) makeReduceBins() error {
	mns, err := c.mftclnt.GetTasksByStatus(ftclnt.DONE)
	if err != nil {
		return err
	}
	obins, err := c.mftclnt.GetTaskOutputs(mns)
	if err != nil {
		return err
	}

	db.DPrintf(db.MR_COORD, "makeReduceBins: obins(%d) %v", len(obins), obins)

	rns := make([]ftclnt.TaskId, c.nreducetask)
	reduceBinIn := make(map[ftclnt.TaskId]Bin, c.nreducetask)
	for i, _ := range rns {
		rns[i] = ftclnt.TaskId(i)
		reduceBinIn[rns[i]] = make(Bin, c.nmaptask)
	}
	for j, obin := range obins {
		for i, s := range obin {
			reduceBinIn[rns[i]][j] = s
		}
	}

	db.DPrintf(db.MR_COORD, "makeReduceBins %d: reduceBinIn %v", len(reduceBinIn), reduceBinIn)

	rnsError, err := c.rftclnt.GetTasksByStatus(ftclnt.ERROR)
	if err != nil {
		return err
	}

	if len(rnsError) > 0 {
		return c.updateReducers(rnsError, reduceBinIn)
	} else {
		return c.createReducers(reduceBinIn)
	}
}

func (c *Coord) Work() {
	db.DPrintf(db.MR_COORD, "Try acquire leadership coord %v job %v", c.ProcEnv().GetPID(), c.job)

	// Try to become the leading coordinator.
	if err := c.leaderclnt.LeadAndFence(nil, []string{JobDir(c.jobRoot, c.job)}); err != nil {
		db.DFatalf("LeadAndFence err %v", err)
	}

	db.DPrintf(db.ALWAYS, "leader %s nmap %v nreduce %v\n", c.job, c.nmaptask, c.nreducetask)

	f := c.leaderclnt.Fence()

	c.mftclnt = ftclnt.NewFtTaskClnt[Bin, Bin](c.FsLib, c.mftid, &f)
	c.rftclnt = ftclnt.NewFtTaskClnt[TreduceTask, Bin](c.FsLib, c.rftid, &f)

	if err := c.mftclnt.Fence(&f); err != nil {
		db.DFatalf("Fence mapper err %v", err)
	}
	if err := c.rftclnt.Fence(&f); err != nil {
		db.DFatalf("Fence reducer err %v", err)
	}

	var err error
	ch := make(chan ftmgr.Tresult[[]byte, []byte])
	c.mcoord, err = fttaskmgr.NewFtTaskCoord[[]byte, []byte](c.SigmaClnt, c.mftclnt.AsRawClnt(), ch)
	c.rcoord, err = fttaskmgr.NewFtTaskCoord[[]byte, []byte](c.SigmaClnt, c.rftclnt.AsRawClnt(), ch)

	crash.Failer(c.FsLib, crash.MRCOORD_CRASH, func(e crash.Tevent) {
		crash.CrashMsg(c.stat.String())
	})
	crash.Failer(c.FsLib, crash.MRCOORD_PARTITION, func(e crash.Tevent) {
		crash.PartitionNamed(c.FsLib)
	})

	start := time.Now()
	if n, err := c.mftclnt.MoveTasksByStatus(ftclnt.WIP, ftclnt.TODO); err != nil {
		db.DFatalf("RecoverTasks mapper err %v", err)
	} else {
		spstats.Store(&c.stat.NrecoverMap, int64(n))
		db.DPrintf(db.MR_COORD, "Recover %d WIP map tasks took %v", n, time.Since(start))
	}

	start = time.Now()
	if n, err := c.rftclnt.MoveTasksByStatus(ftclnt.WIP, ftclnt.TODO); err != nil {
		db.DFatalf("RecoverTasks reducer err %v", err)
	} else {
		c.stat.NrecoverReduce.Store(int64(n))
		db.DPrintf(db.MR_COORD, "Recover %d WIP reduce tasks took %v", n, time.Since(start))
	}

	c.doRestart()

	m, err := c.mftclnt.GetNTasks(ftclnt.DONE)
	if err != nil {
		db.DFatalf("NtaskDone mappers err %v\n", err)
	}
	r, err := c.rftclnt.GetNTasks(ftclnt.DONE)
	if err != nil {
		db.DFatalf("NtaskDone reducers err %v\n", err)
	}

	start = time.Now()
	if int(m+r) < c.nmaptask+c.nreducetask {

		wg := &sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			st, err := c.mcoord.ExecuteTasks(c.mapperProc)
			if err != nil {
				db.DFatalf("ExecuteTasks: mcoord err %v", err)
			}
			n := st.NumDone + st.NumError + st.NumTodo + st.NumWip
			spstats.Inc(&c.stat.Ntask, int64(n))
		}()
		go func() {
			defer wg.Done()
			st, err := c.rcoord.ExecuteTasks(c.reducerProc)
			if err != nil {
				db.DFatalf("ExecuteTasks: rcoord err %v", err)
			}
			n := st.NumDone + st.NumError + st.NumTodo + st.NumWip
			spstats.Inc(&c.stat.Ntask, int64(n))
		}()

		// Consume results as they arrive, and (if enabled) watch for
		// stragglers to back up in parallel with the two phases above.
		go c.processResult(ch, m, r)
		if c.specEnabled {
			go c.speculate(ch)
		}

		// Wait for both phases to finish, then stop the speculation loop.
		wg.Wait()
		close(c.specDone)
		// Deliberately not closing ch: a task can now be marked DONE by
		// whichever of {original, speculative backup} finishes first, while
		// the other attempt's own result (from fttaskmgr's internal
		// runTask/waitForTask, for the original, or runBackupMap/Reduce
		// above, for a backup) may still arrive afterwards. processResult
		// discards those late/duplicate results safely; closing ch here
		// could instead panic on a send-after-close race. The leaked
		// processResult goroutine is harmless -- it's reclaimed when this
		// proc exits below.

		// double check we are done
		m, err = c.mftclnt.GetNTasks(ftclnt.DONE)
		if err != nil {
			db.DFatalf("NtaskDone mappers err %v\n", err)
		}
		r, err = c.rftclnt.GetNTasks(ftclnt.DONE)
		if err != nil {
			db.DFatalf("NtaskDone reducers err %v\n", err)
		}
	}

	if int(m+r) < c.nmaptask+c.nreducetask {
		db.DFatalf("job isn't done %v+%v != %v+%v", m, r, c.nmaptask, c.nreducetask)
	}

	db.DPrintf(db.ALWAYS, "job done stat %v", &c.stat)

	db.DPrintf(db.ALWAYS, "E2e bench took %v", time.Since(start))
	JobDone(c.FsLib, c.jobRoot, c.job)

	stro := spstats.NewTcounterSnapshot()
	stro.FillCounters(&c.stat)
	stro.MergeCounters(c.ProcAPI.Stats())
	c.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK", stro))
	defer c.perf.Done()
}

func (c *Coord) processResult(ch <-chan ftmgr.Tresult[[]byte, []byte], m, r int32) {
	db.DPrintf(db.MR_COORD, "processResults %d %d", m, r)
	nM := int(m)
	nR := int(r)
	nRestart := 0

	// we may recovery with all mappers done and we should kick off
	// the reducers
	if nM >= c.nmaptask {
		if err := c.makeReduceBins(); err != nil {
			db.DFatalf("ReduceBins err %v", err)
		}
	}
	ts := make(map[ftclnt.TaskId]bool)
	tsR := make(map[ftclnt.TaskId]bool)
	for res := range ch {
		db.DPrintf(db.MR_COORD, "processResult: res %v", res)
		if res.Err == nil && res.Status.IsStatusOK() {
			if c.maliciousMapper > 0 && res.Proc.GetProgram() == MALICIOUS_MAPPER_BIN {
				// If running with malicious mapper, then exit status should not be OK.
				// The task should be restarted automatically by the MR FT
				// infrastructure.  If the exit status *was* OK, then the output files
				// won't match, because the malicious mapper doesn't actually do the map
				// (it just touches some buckets it shouldn't have access to). Because of
				// this, letting the coordinator proceed by marking the task as done
				// should cause the test to fail.
				db.DPrintf(db.ERROR, "!!! WARNING: MALICIOUS MAPPER SUCCEEDED !!!")
			}
			r, err := NewResult(res.Status.Data())
			if err != nil {
				db.DFatalf("NewResult %v err %v", res.Status.Data(), err)
			}
			r.MsOuter = res.Ms.Milliseconds()
			db.DPrintf(db.MR_COORD, "Task results %v", r)

			// If a sibling attempt (a speculative backup, or -- for map
			// tasks -- an earlier completion before a restart-triggered
			// redo) already won this task, this is a late loser: discard it
			// instead of overwriting the winner's stored output.
			if (r.IsM && ts[res.Id]) || (!r.IsM && tsR[res.Id]) {
				db.DPrintf(db.MR_COORD, "processResult: discarding late/speculative result for already-finished task %v", res.Id)
				continue
			}

			// mark task as done
			start := time.Now()
			encoded, err := ftclnt.Encode(r.OutBin)
			if err != nil {
				db.DFatalf("Encode %v err %v", r.OutBin, err)
			}
			if err := res.Ftclnt.AddTaskOutputs([]ftclnt.TaskId{res.Id}, [][]byte{encoded}, true); err != nil {
				db.DFatalf("MarkDone %v done err %v", res.Id, err)
			}
			db.DPrintf(db.MR_COORD, "MarkDone latency: lat %v", time.Since(start))
			if err := c.AppendFileJson(MRstats(c.jobRoot, c.job), r); err != nil {
				db.DFatalf("Appendfile %v err %v", MRstats(c.jobRoot, c.job), err)
			}
			if r.IsM {
				ts[res.Id] = true
				c.taskWon(res.Id, res.Proc.GetPid(), res.Ms, true)
				nM += 1
				if nM >= c.nmaptask { // kick off reducers?
					if err := c.makeReduceBins(); err != nil {
						db.DFatalf("ReduceBins err %v", err)
					}
				}
			} else {
				tsR[res.Id] = true
				c.taskWon(res.Id, res.Proc.GetPid(), res.Ms, false)
				nR += 1
				if nR >= c.nreducetask {
					db.DPrintf(db.MR_COORD, "processResult: SubmittedLastTask")
					c.mftclnt.SubmittedLastTask()
					c.rftclnt.SubmittedLastTask()
				}
			}
			db.DPrintf(db.ALWAYS, "tasks done %d/%d\n", nM+nR, c.nmaptask+c.nreducetask)
		} else {
			db.DPrintf(db.MR, "Task failed %v status %v", res.Id, res.Status)
			if res.Status != nil && res.Status.Msg() == RESTART {
				// reducer indicates to run some mappers again
				s := newStringSlice(res.Status.Data().([]interface{}))
				c.restart(s, res.Id)
				nRestart += 1
				c.evictSiblings(res.Id, res.Proc.GetPid(), false)
			} else { // if failure but not restart, rerun task immediately again
				if err := res.Ftclnt.MoveTasks([]ftclnt.TaskId{res.Id}, ftclnt.TODO); err != nil {
					db.DFatalf("MarkRunnable %v err %v", res.Id, err)
				}
				c.evictSiblings(res.Id, res.Proc.GetPid(), res.Ftclnt == c.mftclnt.AsRawClnt())
			}
			c.stat.Nfail.Add(1)
		}
		if nR+nRestart >= c.nreducetask { // Run mappers again for some errored reducers?
			nM = 0
			nRestart = 0
			ts = make(map[ftclnt.TaskId]bool)
			c.resetSpeculation()
			c.doRestart()
		}
	}
}
