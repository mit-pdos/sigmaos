package mr

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/electclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	MIN    = "name/s3/~ip/input/"
	MRDIR  = "name/mr"
	MDIR   = MRDIR + "/m"
	RDIR   = MRDIR + "/r"
	RIN    = MRDIR + "-rin/"
	ROUT   = MRDIR + "/" + "mr-out-"
	TIP    = "-tip/"
	DONE   = "-done/"
	NEXT   = "-next/"
	NCOORD = 3

	MLOCALDIR    = "/mr/"
	MLOCALSRV    = np.UX + "/~ip" // must end without /
	MLOCALMR     = MLOCALSRV + MLOCALDIR
	MLOCALPREFIX = MLOCALMR + "m-"

	RESTART = "restart" // restart message from reducer
)

func Moutdir(name string) string {
	return MLOCALPREFIX + name
}

func mshardfile(name string, r int) string {
	return Moutdir(name) + "/r-" + strconv.Itoa(r)
}

func shardtarget(server, name string, r int) string {
	return np.UX + "/" + server + MLOCALDIR + "m-" + name + "/r-" + strconv.Itoa(r) + "/"
}

func symname(r string, name string) string {
	return RIN + "/" + r + "/m-" + name
}

//
// mr_test puts names of input files in MDIR and launches a
// coordinator proc to run the MR job.  The actual input files live in
// MIN.
//
// The coordinator creates one thread per input file, which looks for
// a file name in MDIR. If thread finds a name, it claims it by
// renaming it into MDIR+TIP to record that a task for name is in
// progress.  Then, the thread creates a mapper proc (task) to process
// the input file.  Mapper i creates <r> output shards, one for each
// reducer.  Once the mapper completes an output shard, it creates a
// symlink in dir RIN+/r, which contains the pathname for the output
// shard.  If the mapper proc successfully exits, the thread renames
// the task from MDIR+TIP into the dir MDIR+DONE, to record that this
// mapper task has completed.
//
// The coordinator creates one thread per reducer, which grabs <r>
// from RDIR, and records in RDIR+TIP that reducer <r> is in progress.
// The thread creates a reducer proc that looks in dir RIN+/r for
// symlinks to process (one symlink per mapper task). The symlinks
// contain the pathname where the mapper puts its shard for this
// reducer.  The reducer writes it output to ROUT+<r>.  If the reducer
// task exits successfully, the thread renames RDIR+TIP+r into
// RDIR+DONE, to record that this reducer task has completed.
//

func InitCoordFS(fsl *fslib.FsLib, nreducetask int) {
	for _, n := range []string{MRDIR, MDIR, RDIR, RIN, MDIR + TIP, RDIR + TIP, MDIR + DONE, RDIR + DONE, MDIR + NEXT, RDIR + NEXT} {
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Make task and input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := RDIR + "/" + strconv.Itoa(r)
		if _, err := fsl.PutFile(n, 0777, np.OWRITE, []byte{}); err != nil {
			db.DFatalf("Putfile %v err %v\n", n, err)
		}
		n = RIN + "/" + strconv.Itoa(r)
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Remove intermediate dir. XXX If we run with many machine
	// this must happen on each ux.
	fsl.RmDir(MLOCALMR)
	if err := fsl.MkDir(MLOCALMR, 0777); err != nil {
		db.DFatalf("Mkdir %v err %v\n", MLOCALMR, err)
	}
}

type Coord struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	nmaptask    int
	nreducetask int
	crash       int
	mapperbin   string
	reducerbin  string
	electclnt   *electclnt.ElectClnt
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) != 5 {
		return nil, errors.New("MakeCoord: too few arguments")
	}
	w := &Coord{}
	w.FsLib = fslib.MakeFsLib("coord-" + proc.GetPid().String())

	m, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("MakeCoord: nmaptask %v isn't int", args[0])
	}
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("MakeCoord: nreducetask %v isn't int", args[1])
	}
	w.nmaptask = m
	w.nreducetask = n
	w.mapperbin = args[2]
	w.reducerbin = args[3]

	c, err := strconv.Atoi(args[4])
	if err != nil {
		return nil, fmt.Errorf("MakeCoord: crash %v isn't int", args[3])
	}
	w.crash = c

	w.ProcClnt = procclnt.MakeProcClnt(w.FsLib)

	w.Started()

	w.electclnt = electclnt.MakeElectClnt(w.FsLib, MRDIR+"/coord-leader", 0)

	crash.Crasher(w.FsLib)

	return w, nil
}

func (c *Coord) task(bin string, args []string) (*proc.Status, error) {
	p := proc.MakeProc(bin, args)
	if c.crash > 0 {
		p.AppendEnv("SIGMACRASH", strconv.Itoa(c.crash))
	}
	db.DPrintf("MR", "spawn task %v %v\n", p.Pid, args)
	err := c.Spawn(p)
	if err != nil {
		return nil, err
	}
	status, err := c.WaitExit(p.Pid)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (c *Coord) mapper(task string) (*proc.Status, error) {
	input := MIN + task
	return c.task(c.mapperbin, []string{strconv.Itoa(c.nreducetask), input})
}

func (c *Coord) reducer(task string) (*proc.Status, error) {
	in := RIN + "/" + task
	out := ROUT + task
	return c.task(c.reducerbin, []string{in, out, strconv.Itoa(c.nmaptask)})
}

func (c *Coord) claimEntry(dir string, st *np.Stat) (string, error) {
	from := dir + "/" + st.Name
	if err := c.Rename(from, dir+TIP+"/"+st.Name); err != nil {
		if np.IsErrUnreachable(err) { // partitioned?
			return "", err
		}
		// another thread claimed the task before us
		return "", nil
	}
	return st.Name, nil
}

func (c *Coord) getTask(dir string) (string, error) {
	t := ""
	stopped, err := c.ProcessDir(dir, func(st *np.Stat) (bool, error) {
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
	t  string
	ok bool
	ms int64
}

func (c *Coord) doneTasks(dir string) int {
	sts, err := c.GetDir(dir)
	if err != nil {
		db.DFatalf("doneTasks err %v\n", err)
	}
	return len(sts)
}

func (c *Coord) runTask(ch chan Tresult, dir string, t string, f func(string) (*proc.Status, error)) {
	start := time.Now()
	status, err := f(t)
	ms := time.Since(start).Milliseconds()
	if err == nil && status.IsStatusOK() {
		// mark task as done
		s := dir + TIP + "/" + t
		d := dir + DONE + "/" + t
		if err := c.Rename(s, d); err != nil {
			db.DFatalf("rename task done %v to %v err %v\n", s, d, err)
		}
		ch <- Tresult{t, true, ms}
	} else { // task failed; make it runnable again
		if status != nil && status.Msg() == RESTART {
			// reducer indicates to run some mappers again
			s := mkStringSlice(status.Data().([]interface{}))
			c.restart(s, t)
		} else { // if failure but not restart, rerun task immediately again
			to := dir + "/" + t
			db.DPrintf("MR", "task %v failed %v err %v\n", t, status, err)
			if err := c.Rename(dir+TIP+"/"+t, to); err != nil {
				db.DFatalf("rename to runnable %v err %v\n", to, err)
			}
		}
		ch <- Tresult{t, false, ms}
	}
}

func mkStringSlice(data []interface{}) []string {
	s := make([]string, 0, len(data))
	for _, o := range data {
		s = append(s, o.(string))
	}
	return s
}

func (c *Coord) startTasks(ch chan Tresult, dir string, f func(string) (*proc.Status, error)) int {
	n := 0
	for {
		t, err := c.getTask(dir)
		if err != nil {
			db.DFatalf("getTask %v err %v\n", dir, err)
		}
		if t == "" {
			break
		}
		n += 1
		go c.runTask(ch, dir, t, f)
	}
	return n
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
		sym := symname(task, f)
		if err := c.Remove(sym); err != nil {
			db.DPrintf(db.ALWAYS, "remove %v err %v\n", sym, err)
		}

		// Record that we have to rerun mapper f
		n := MDIR + NEXT + "/" + f
		if _, err := c.PutFile(n, 0777, np.OWRITE, []byte(n)); err != nil {
			db.DPrintf(db.ALWAYS, "PutFile %v err %v\n", n, err)
		}
	}
	// Record that we have to rerun reducer t
	n := RDIR + NEXT + "/" + task
	if _, err := c.PutFile(n, 0777, np.OWRITE, []byte(n)); err != nil {
		db.DPrintf(db.ALWAYS, "PutFile %v err %v\n", n, err)
	}
}

// Mark all restart tasks as runnable
func (c *Coord) doRestart() bool {
	n, err := c.MoveFiles(MDIR+NEXT, MDIR)
	if err != nil {
		db.DFatalf("MoveFiles %v err %v\n", MDIR, err)
	}
	m, err := c.MoveFiles(RDIR+NEXT, RDIR)
	if err != nil {
		db.DFatalf("MoveFiles %v err %v\n", RDIR, err)
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
func (c *Coord) Round() {
	ch := make(chan Tresult)
	for m := 0; ; m-- {
		m += c.startTasks(ch, MDIR, c.mapper)
		m += c.startTasks(ch, RDIR, c.reducer)
		if m <= 0 {
			break
		}
		res := <-ch
		db.DPrintf(db.ALWAYS, "%v ok %v ms %d\n", res.t, res.ok, res.ms)
	}
}

func (c *Coord) Work() {
	// Try to become the leading coordinator.  If we get
	// partitioned, we cannot write the todo directories either,
	// so need to set a fence.
	c.electclnt.AcquireLeadership(nil)

	db.DPrintf(db.ALWAYS, "leader nmap %v nreduce %v\n", c.nmaptask, c.nreducetask)

	c.recover(MDIR)
	c.recover(RDIR)
	c.doRestart()

	for n := 0; ; {
		db.DPrintf(db.ALWAYS, "run round %d\n", n)
		c.Round()
		if !c.doRestart() {
			break
		}
	}

	// double check we are done
	n := c.doneTasks(MDIR + DONE)
	n += c.doneTasks(RDIR + DONE)
	if n < c.nmaptask+c.nreducetask {
		db.DFatalf("job isn't done %v", n)
	}

	db.DPrintf(db.ALWAYS, "job done\n")

	c.Exited(proc.MakeStatus(proc.StatusOK))
}
