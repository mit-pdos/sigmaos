// The mr package implements a MapReduce library using sigmaos procs.
package mr

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fttasks"
	"sigmaos/leaderclnt"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	NCOORD               = 3
	RESTART              = "restart" // restart message from reducer
	MALICIOUS_MAPPER_BIN = "mr-m-malicious"
)

// mr_test puts pathnames of input files (split into bins) in
// MR/<job>/m/ and creates an fttask task for each one of them.  It
// also creates a number of reducer tasks (one for each reducer).
//
// The coordinator claims tasks and start procs for them, which
// process the claimed task.  Mapper i creates <r> output shards, one
// for each reducer.  Once the mapper completes an output shard, it
// creates a symlink in dir <job>/<r>/, which contains the pathname
// for the mapper's output shard for reducer <r>.
//
// If a mapper or reducer proc successfully exits, the coordinator
// marks the task as done.  If it fails, the coordinator will make the
// task runnable again and start new mapper/reducer procs to process
// the task.  If the coordinator fails, another coordinator will take
// over and claim tasks.

type Coord struct {
	*sigmaclnt.SigmaClnt
	mft             *fttasks.FtTasks
	rft             *fttasks.FtTasks
	job             string
	nmaptask        int
	nreducetask     int
	crash           int64
	maliciousMapper uint64
	linesz          string
	mapperbin       string
	reducerbin      string
	leaderclnt      *leaderclnt.LeaderClnt
	outdir          string
	intOutdir       string
	done            int32
	memPerTask      proc.Tmem
	asyncrw         bool
}

func NewCoord(args []string) (*Coord, error) {
	if len(args) != 10 {
		return nil, errors.New("NewCoord: wrong number of arguments")
	}
	c := &Coord{}
	c.job = args[0]
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.MR, "Made fslib job %v", c.job)
	c.SigmaClnt = sc
	m, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nmaptask %v isn't int", args[1])
	}
	n, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nreducetask %v isn't int", args[2])
	}
	c.nmaptask = m
	c.nreducetask = n
	c.mapperbin = args[3]
	c.reducerbin = args[4]

	ctime, err := strconv.Atoi(args[5])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: crash %v isn't int", args[5])
	}
	c.crash = int64(ctime)

	malmap, err := strconv.Atoi(args[9])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: maliciousMapper %v isn't int", args[9])
	}
	c.maliciousMapper = uint64(malmap)

	c.linesz = args[6]

	mem, err := strconv.Atoi(args[7])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nreducetask %v isn't int", args[2])
	}
	c.memPerTask = proc.Tmem(mem)
	asyncrw, err := strconv.ParseBool(args[8])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: can't parse asyncrw %v", args[8])
	}
	c.asyncrw = asyncrw

	b, err := c.GetFile(JobOutLink(c.job))
	if err != nil {
		db.DFatalf("Error GetFile JobOutLink [%v]: %v", JobOutLink(c.job), err)
	}
	c.outdir = string(b)

	b, err = c.GetFile(JobIntOutLink(c.job))
	if err != nil {
		db.DFatalf("Error GetFile JobIntOutLink: %v", err)
	}
	c.intOutdir = string(b)

	c.mft, err = fttasks.NewFtTasks(c.FsLib, MRDIRTOP, filepath.Join(c.job, "/mtasks"))
	if err != nil {
		db.DFatalf("NewFtTasks mtasks %v", err)
	}
	c.rft, err = fttasks.NewFtTasks(c.FsLib, MRDIRTOP, filepath.Join(c.job, "/rtasks"))
	if err != nil {
		db.DFatalf("NewFtTasks rtasks %v", err)
	}

	c.Started()

	c.leaderclnt, err = leaderclnt.NewLeaderClnt(c.FsLib, LeaderElectDir(c.job)+"/coord-leader", 0)
	if err != nil {
		return nil, fmt.Errorf("NewCoord: NewLeaderclnt err %v", err)
	}

	crash.Crasher(c.FsLib)

	return c, nil
}

func (c *Coord) newTask(bin string, args []string, mb proc.Tmem, allowedPaths []string) *proc.Proc {
	pid := sp.GenPid(bin + "-" + c.job)
	p := proc.NewProcPid(pid, bin, args)
	//	if mb > 0 {
	//		p.AppendEnv("GOMEMLIMIT", strconv.Itoa(int(mb)*1024*1024))
	//	}
	p.SetMem(mb)
	if c.crash > 0 {
		p.SetCrash(c.crash)
	}
	return p
}

func (c *Coord) mapperProc(task string) *proc.Proc {
	input := c.mft.TaskPathName(task)
	allowedPaths := []string{sp.NAMED, filepath.Join(sp.SCHEDD, "*"), filepath.Join(sp.S3, "*"), filepath.Join(sp.UX, "*")}
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
	return c.newTask(mapperbin, []string{c.job, strconv.Itoa(c.nreducetask), input, c.intOutdir, c.linesz, strconv.FormatBool(c.asyncrw)}, c.memPerTask, allowedPaths)
}

type TreduceTask struct {
	Task string `json:"Task"`
}

func (c *Coord) reducerProc(tn string) *proc.Proc {
	t := &TreduceTask{}
	if err := c.rft.ReadTask(tn, t); err != nil {
		db.DFatalf("ReadTask %v err %v", tn, err)
	}
	in := ReduceIn(c.job) + "/" + t.Task
	outlink := ReduceOut(c.job) + t.Task
	outTarget := ReduceOutTarget(c.outdir, c.job) + t.Task
	allowedPaths := []string{sp.NAMED, filepath.Join(sp.SCHEDD, "*"), filepath.Join(sp.S3, "*"), filepath.Join(sp.UX, "*")}
	return c.newTask(c.reducerbin, []string{in, outlink, outTarget, strconv.Itoa(c.nmaptask), strconv.FormatBool(c.asyncrw)}, c.memPerTask, allowedPaths)
}

type Tresult struct {
	t   string
	ok  bool
	ms  int64
	msg string
	res *Result
}

func (c *Coord) waitForTask(ft *fttasks.FtTasks, start time.Time, ch chan Tresult, p *proc.Proc, t string) {
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
		// mark task as done
		start := time.Now()
		if err := ft.MarkDone(t); err != nil {
			db.DFatalf("MarkDone %v done err %v", t, err)
		}
		db.DPrintf(db.MR, "MarkDone task latency: %v", start)
		r := NewResult(status.Data())
		r.MsOuter = ms
		ch <- Tresult{t, true, ms, status.Msg(), r}
	} else { // task failed; make it runnable again
		if status != nil && status.Msg() == RESTART {
			// reducer indicates to run some mappers again
			s := newStringSlice(status.Data().([]interface{}))
			c.restart(s, t)
		} else { // if failure but not restart, rerun task immediately again
			if err := ft.MarkRunnable(t); err != nil {
				db.DFatalf("MarkRunnable %v err %v", t, err)
			}
		}
		ch <- Tresult{t, false, ms, "", nil}
	}
}

func (c *Coord) runTasks(ft *fttasks.FtTasks, ch chan Tresult, taskNames []string, f func(string) *proc.Proc) {
	db.DPrintf(db.MR, "runTasks %v", taskNames)
	for _, tn := range taskNames {
		t := f(tn)
		db.DPrintf(db.MR, "prep to spawn task %v %v", t.GetPid(), t.Args)
		start := time.Now()
		err := c.Spawn(t)
		if err != nil {
			db.DFatalf("Err spawn task: %v", err)
		}
		go c.waitForTask(ft, start, ch, t, tn)
	}
}

func newStringSlice(data []interface{}) []string {
	s := make([]string, 0, len(data))
	for _, o := range data {
		s = append(s, o.(string))
	}
	return s
}

func (c *Coord) startTasks(ft *fttasks.FtTasks, ch chan Tresult, f func(string) *proc.Proc) int {
	start := time.Now()
	tns, err := ft.GetTasks()
	if err != nil {
		db.DFatalf("startTasks err %v\n", err)
	}
	db.DPrintf(db.MR, "startTasks %v time: %v", tns, time.Since(start))
	c.runTasks(ft, ch, tns, f)
	return len(tns)
}

// A reducer failed because it couldn't read its input file; we must
// restart mapper.  We let all mappers and reducers finish, before
// starting a new round for the mappers and reducers that must be
// restarted, which avoids restarting a mapper several times (because
// several reducers may ask the mapper to be restarted).
func (c *Coord) restart(files []string, task string) {
	db.DPrintf(db.ALWAYS, "restart %v and %v\n", files, task)
	for _, f := range files {
		// Remove symfile so that when coordinator restarts
		// reducers, they wait for the mappers to make new
		// symfiles.
		sym := symname(c.job, task, f)
		if err := c.Remove(sym); err != nil {
			db.DPrintf(db.ALWAYS, "remove %v err %v\n", sym, err)
		}
		// Record that we have to rerun mapper f
		if err := c.mft.MarkError(f); err != nil {
			db.DPrintf(db.ALWAYS, "restart %v err %v\n", f, err)
		}
	}
	// Record that we have to rerun reducer task
	if err := c.rft.MarkError(task); err != nil {
		db.DPrintf(db.ALWAYS, "restart reducer %v err %v\n", task, err)
	}
}

// Mark all error-ed tasks as runnable
func (c *Coord) doRestart() bool {
	n, err := c.mft.Restart()
	if err != nil {
		db.DFatalf("Restart mappers err %v\n", err)
	}
	m, err := c.rft.Restart()
	if err != nil {
		db.DFatalf("Restart reducers err %v\n", err)
	}
	if n+m > 0 {
		db.DPrintf(db.ALWAYS, "restarted %d tasks\n", n+m)
	}
	return n+m > 0
}

// XXX do something for stragglers?
func (c *Coord) Round(ttype string) {
	mapsDone := false
	start := time.Now()
	ch := make(chan Tresult)
	for m := 0; ; m-- {
		if ttype == "map" {
			m += c.startTasks(c.mft, ch, c.mapperProc)
		} else if ttype == "reduce" {
			m += c.startTasks(c.rft, ch, c.reducerProc)
		} else if ttype == "all" {
			m += c.startTasks(c.mft, ch, c.mapperProc)
			db.DPrintf(db.MR, "startTasks mappers %v", m)
			m += c.startTasks(c.rft, ch, c.reducerProc)
			db.DPrintf(db.MR, "startTasks add reducers %v", m)
		} else {
			db.DFatalf("Unknown ttype: %v", ttype)
		}
		if m <= 0 {
			break
		}
		res := <-ch
		db.DPrintf(db.MR, "Round: task done %v ok %v msInner %d msOuter %d msg %v res %v\n", res.t, res.ok, res.res.MsInner, res.res.MsOuter, res.msg, res.res)
		if res.ok {
			if err := c.AppendFileJson(MRstats(c.job), res.res); err != nil {
				db.DFatalf("Appendfile %v err %v\n", MRstats(c.job), err)
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
	db.DPrintf(db.MR, "Try acquire leadership coord %v job %v", c.ProcEnv().GetPID(), c.job)

	// Try to become the leading coordinator.
	if err := c.leaderclnt.LeadAndFence(nil, []string{JobDir(c.job)}); err != nil {
		db.DFatalf("LeadAndFence err %v", err)
	}

	db.DPrintf(db.ALWAYS, "leader %s nmap %v nreduce %v\n", c.job, c.nmaptask, c.nreducetask)
	start := time.Now()
	if err := c.mft.RecoverTasks(); err != nil {
		db.DFatalf("RecoverTasks mapper err %v", err)
	}
	start = time.Now()
	db.DPrintf(db.MR, "Recover map tasks took %v", time.Since(start))
	if err := c.rft.RecoverTasks(); err != nil {
		db.DFatalf("RecoverTasks reducer err %v", err)
	}
	db.DPrintf(db.MR, "Recover reduce tasks took %v", time.Since(start))
	start = time.Now()
	c.doRestart()
	db.DPrintf(db.MR, "doRestart took %v", time.Since(start))
	jobStart := time.Now()

	for n := 0; ; {
		db.DPrintf(db.ALWAYS, "run round %d\n", n)
		//		c.Round("all")
		start := time.Now()
		c.Round("map")
		n, err := c.mft.NTaskDone()
		if err != nil {
			db.DFatalf("NtaskDone err %v\n", err)
		}
		if n == c.nmaptask {
			ms := time.Since(start).Milliseconds()
			db.DPrintf(db.ALWAYS, "map phase took %vms\n", ms)
			c.Round("reduce")
		}
		if !c.doRestart() {
			break
		}
	}

	// double check we are done
	n, err := c.mft.NTaskDone()
	if err != nil {
		db.DFatalf("NtaskDone mappers err %v\n", err)
	}
	m, err := c.rft.NTaskDone()
	if err != nil {
		db.DFatalf("NtaskDone reducers err %v\n", err)
	}
	if n+m < c.nmaptask+c.nreducetask {
		db.DFatalf("job isn't done %v+%v != %v+%v", n, m, c.nmaptask, c.nreducetask)
	}

	db.DPrintf(db.ALWAYS, "job done\n")

	atomic.StoreInt32(&c.done, 1)

	db.DPrintf(db.ALWAYS, "E2e bench took %v", time.Since(jobStart))
	JobDone(c.FsLib, c.job)

	c.ClntExitOK()
}
