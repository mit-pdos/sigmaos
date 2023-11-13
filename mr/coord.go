package mr

import (
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/leaderclnt"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	MR       = "/mr/"
	MRDIRTOP = "name/" + MR
	OUTLINK  = "output"
	JOBSEM   = "jobsem"

	TIP  = "-tip/"
	DONE = "-done/"
	NEXT = "-next/"

	NCOORD = 3

	MLOCALSRV = sp.UX + "/~local" // must end without /
	MLOCALDIR = MLOCALSRV + MR

	RESTART = "restart" // restart message from reducer
)

//
// mr_test puts pathnames of input files (split into bins) in
// MR/<job>/m/ and launches a coordinator proc to run the MR job <job>.
//
// The coordinator creates one thread per bin, which looks for a bin
// (a chunk of an input file: a pathname, offset, size) name in /m. If
// thread finds a bin, it claims it by renaming it into the dir TIP to
// record that a task for name is in progress.  Then, the thread
// creates a mapper proc (task) to process the bin.  Mapper i creates
// <r> output shards, one for each reducer.  Once the mapper completes
// an output shard, it creates a symlink in dir <job>/<r>/, which
// contains the pathname for the mapper's output shard for reducer
// <r>.  If the mapper proc successfully exits, the thread renames the
// task from TIP into the dir DONE, to record that this mapper task
// has completed.  The coordinator follows a similar plan for reducing
// the shards generated by the mapper procs.

type Coord struct {
	*sigmaclnt.SigmaClnt
	job         string
	nmaptask    int
	nreducetask int
	crash       int64
	linesz      string
	mapperbin   string
	reducerbin  string
	leaderclnt  *leaderclnt.LeaderClnt
	outdir      string
	done        int32
	memPerTask  proc.Tmem
}

func NewCoord(args []string) (*Coord, error) {
	if len(args) != 8 {
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

	c.linesz = args[6]

	mem, err := strconv.Atoi(args[7])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nreducetask %v isn't int", args[2])
	}
	c.memPerTask = proc.Tmem(mem)

	b, err := c.GetFile(JobOutLink(c.job))
	if err != nil {
		db.DFatalf("Error GetFile JobOutLink: %v", err)
	}
	c.outdir = string(b)

	c.Started()

	c.leaderclnt, err = leaderclnt.NewLeaderClnt(c.FsLib, JobDir(c.job)+"/coord-leader", 0)
	if err != nil {
		return nil, fmt.Errorf("NewCoord: NewLeaderclnt err %v", err)
	}

	crash.Crasher(c.FsLib)

	return c, nil
}

func (c *Coord) newTask(bin string, args []string, mb proc.Tmem) *proc.Proc {
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
	input := MapTask(c.job) + TIP + task
	return c.newTask(c.mapperbin, []string{c.job, strconv.Itoa(c.nreducetask), input, c.linesz}, c.memPerTask)
}

func (c *Coord) reducerProc(task string) *proc.Proc {
	in := ReduceIn(c.job) + "/" + task
	outlink := ReduceOut(c.job) + task
	outTarget := ReduceOutTarget(c.outdir, c.job) + task
	return c.newTask(c.reducerbin, []string{in, outlink, outTarget, strconv.Itoa(c.nmaptask)}, c.memPerTask)
}

func (c *Coord) claimEntry(dir string, st *sp.Stat) (string, error) {
	from := dir + "/" + st.Name
	if err := c.Rename(from, dir+TIP+"/"+st.Name); err != nil {
		if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
			return "", err
		}
		// another thread claimed the task before us
		return "", nil
	}
	return st.Name, nil
}

func (c *Coord) getTask(dir string) (string, error) {
	t := ""
	stopped, err := c.ProcessDir(dir, func(st *sp.Stat) (bool, error) {
		t1, err := c.claimEntry(dir, st)
		if err != nil {
			return false, err
		}
		if t1 == "" {
			return false, nil
		}
		t = t1
		return true, nil

	})
	if err != nil {
		return "", err
	}
	if stopped {
		return t, nil
	}
	return "", nil
}

type Tresult struct {
	t   string
	ok  bool
	ms  int64
	msg string
	res *Result
}

func (c *Coord) doneTasks(dir string) int {
	sts, err := c.GetDir(dir)
	if err != nil {
		db.DFatalf("doneTasks err %v\n", err)
	}
	return len(sts)
}

func (c *Coord) waitForTask(start time.Time, ch chan Tresult, dir string, p *proc.Proc, t string) {
	// Wait for the task to exit.
	status, err := c.WaitExit(p.GetPid())
	// Record end time.
	ms := time.Since(start).Milliseconds()
	if err == nil && status.IsStatusOK() {
		// mark task as done
		inProgress := dir + TIP + "/" + t
		done := dir + DONE + "/" + t
		if err := c.Rename(inProgress, done); err != nil {
			db.DFatalf("rename task done %v to %v err %v\n", inProgress, done, err)
		}
		r := newResult(status.Data())
		ch <- Tresult{t, true, ms, status.Msg(), r}
	} else { // task failed; make it runnable again
		if status != nil && status.Msg() == RESTART {
			// reducer indicates to run some mappers again
			s := newStringSlice(status.Data().([]interface{}))
			c.restart(s, t)
		} else { // if failure but not restart, rerun task immediately again
			to := dir + "/" + t
			db.DPrintf(db.MR, "task %v failed %v err %v\n", t, status, err)
			if err := c.Rename(dir+TIP+"/"+t, to); err != nil {
				db.DFatalf("rename to runnable %v err %v\n", to, err)
			}
		}
		ch <- Tresult{t, false, ms, "", nil}
	}
}

func (c *Coord) runTasks(ch chan Tresult, dir string, taskNames []string, f func(string) *proc.Proc) {
	for _, tn := range taskNames {
		t := f(tn)
		db.DPrintf(db.MR, "prep to spawn task %v %v", t.GetPid(), t.Args)
		start := time.Now()
		err := c.Spawn(t)
		if err != nil {
			db.DFatalf("Err spawn task: %v", err)
		}
		go c.waitForTask(start, ch, dir, t, tn)
	}
}

func newStringSlice(data []interface{}) []string {
	s := make([]string, 0, len(data))
	for _, o := range data {
		s = append(s, o.(string))
	}
	return s
}

func (c *Coord) startTasks(ch chan Tresult, dir string, f func(string) *proc.Proc) int {
	taskNames := []string{}
	for {
		t, err := c.getTask(dir)
		if err != nil {
			db.DFatalf("getTask %v err %v\n", dir, err)
		}
		if t == "" {
			break
		}
		taskNames = append(taskNames, t)
	}
	go c.runTasks(ch, dir, taskNames, f)
	return len(taskNames)
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
		// Record that we have to rerun mapper f.  Mapper may
		// still be in progress, is done, or already has been
		// moved to d.
		s := MapTask(c.job) + DONE + "/" + f
		s1 := MapTask(c.job) + TIP + "/" + f
		d := MapTask(c.job) + NEXT + "/" + f
		if err := c.Rename(s1, d); err != nil {
			db.DPrintf(db.ALWAYS, "rename next  %v to %v err %v\n", s, d, err)
		}
		if err := c.Rename(s, d); err != nil {
			db.DPrintf(db.ALWAYS, "rename next  %v to %v err %v\n", s, d, err)
		}
	}
	// Record that we have to rerun reducer t
	n := ReduceTask(c.job) + NEXT + "/" + task
	if _, err := c.PutFile(n, 0777, sp.OWRITE, []byte(n)); err != nil {
		db.DPrintf(db.ALWAYS, "PutFile %v err %v\n", n, err)
	}
}

// Mark all restart tasks as runnable
func (c *Coord) doRestart() bool {
	n, err := c.MoveFiles(MapTask(c.job)+NEXT, MapTask(c.job))
	if err != nil {
		db.DFatalf("MoveFiles %v err %v\n", MapTask(c.job), err)
	}
	m, err := c.MoveFiles(ReduceTask(c.job)+NEXT, ReduceTask(c.job))
	if err != nil {
		db.DFatalf("MoveFiles %v err %v\n", ReduceTask(c.job), err)
	}
	return n+m > 0
}

// Consider all tasks in progress as failed (too aggressive, but
// correct), and make them runnable
func (c *Coord) recover(dir string) {
	if _, err := c.MoveFiles(dir+TIP, dir); err != nil {
		db.DFatalf("MoveFiles %v err %v\n", dir, err)
	}
}

// XXX do something for stragglers?
func (c *Coord) Round(ttype string) {
	mapsDone := false
	start := time.Now()
	ch := make(chan Tresult)
	for m := 0; ; m-- {
		if ttype == "map" {
			m += c.startTasks(ch, MapTask(c.job), c.mapperProc)
		} else if ttype == "reduce" {
			m += c.startTasks(ch, ReduceTask(c.job), c.reducerProc)
		} else if ttype == "all" {
			m += c.startTasks(ch, MapTask(c.job), c.mapperProc)
			time.Sleep(10 * time.Second)
			m += c.startTasks(ch, ReduceTask(c.job), c.reducerProc)
		} else {
			db.DFatalf("Unknown ttype: %v", ttype)
		}
		if m <= 0 {
			break
		}
		res := <-ch
		db.DPrintf(db.MR, "%v ok %v ms %d msg %v res %v\n", res.t, res.ok, res.ms, res.msg, res.res)
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

	c.recover(MapTask(c.job))
	c.recover(ReduceTask(c.job))
	c.doRestart()

	for n := 0; ; {
		db.DPrintf(db.ALWAYS, "run round %d\n", n)
		//		c.Round("all")
		start := time.Now()
		c.Round("map")
		n := c.doneTasks(MapTask(c.job) + DONE)
		if n == c.nmaptask {
			ms := time.Since(start).Milliseconds()
			db.DPrintf(db.ALWAYS, "map phase took %v ms\n", ms)
			c.Round("reduce")
		}
		if !c.doRestart() {
			break
		}
	}

	// double check we are done
	n := c.doneTasks(MapTask(c.job) + DONE)
	n += c.doneTasks(ReduceTask(c.job) + DONE)
	if n < c.nmaptask+c.nreducetask {
		db.DFatalf("job isn't done %v", n)
	}

	db.DPrintf(db.ALWAYS, "job done\n")

	atomic.StoreInt32(&c.done, 1)

	JobDone(c.FsLib, c.job)

	c.ClntExitOK()
}
