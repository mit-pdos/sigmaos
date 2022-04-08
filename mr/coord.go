package mr

import (
	"errors"
	"fmt"
	"log"
	"path"
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
	INPUTDIR = "name/s3/~ip/input/"
	MRDIR    = "name/mr"
	MDIR     = MRDIR + "/m"
	RDIR     = MRDIR + "/r"
	ROUT     = MRDIR + "/" + "mr-out-"
	TIP      = "-tip/"
	DONE     = "-done/"
	RESTART  = "restart"
	NCOORD   = 3

	MLOCALSRV    = np.UX + "/~ip"
	MLOCALPREFIX = MLOCALSRV + "/m-"
)

func Moutdir(name string) string {
	return MLOCALPREFIX + name
}

func mshardfile(name string, r int) string {
	return Moutdir(name) + "/r-" + strconv.Itoa(r)
}

func shardtarget(server, name string, r int) string {
	return np.UX + "/" + server + "/m-" + name + "/r-" + strconv.Itoa(r) + "/"
}

func symname(r int, name string) string {
	return RDIR + "/" + strconv.Itoa(r) + "/m-" + name
}

//
// mr_test puts names of input files in in MDIR and launches a
// coordinator proc to run the MR job.  The actual files live in
// INPUTDIR.
//
// The coordinator creates one thread per input file, which looks for
// a file name from the MDIR. If thread finds a name, it claims it by
// renaming it into MDIR+TIP to record to that a task for name is in
// progress.  Then the thread creates a mapper proc (task) to process
// the input file.  Mapper i creates <r> output shards, one for each
// reducer.  Once the mapper completes an output shard, it creates a
// symlink in dir RDIR+/r, which contains the pathname for the output
// shard.  If the mapper proc successfully exits, the coordinator
// renames the task from MDIR+TIP into the dir MDIR+DONE, to record
// that this mapper task has completed.
//
// The coordinator creates one thread per reducer, which grabs <r>
// from RDIR, and records in RDIR+TIP that reducer <r> is in progress.
// The thread creates a reducer proc that looks in dir RDIR+/r for
// symlinks to process (one symlink per mapper). The symlinks contain
// the pathname where the mapper puts its shard for this reducer.  The
// reducer writes it output to ROUT+<r>.  If the reducer task exits
// successfully, the coordinator renames RDIR+TIP+r into RDIR+DONE, to
// record that this reducer task has completed.
//

func InitCoordFS(fsl *fslib.FsLib, nreducetask int) {
	for _, n := range []string{MRDIR, MDIR, RDIR, MDIR + TIP, RDIR + TIP, MDIR + DONE, RDIR + DONE} {
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v\n", err)
		}
	}

	// input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := RDIR + "/" + strconv.Itoa(r)
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
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
	a := proc.MakeProc(bin, args)
	if c.crash > 0 {
		a.AppendEnv("SIGMACRASH", strconv.Itoa(c.crash))
	}
	err := c.Spawn(a)
	if err != nil {
		return nil, err
	}
	status, err := c.WaitExit(a.Pid)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (c *Coord) mapper(task string) (*proc.Status, error) {
	input := INPUTDIR + task
	return c.task(c.mapperbin, []string{strconv.Itoa(c.nreducetask), input})
}

func (c *Coord) reducer(task string) (*proc.Status, error) {
	in := RDIR + TIP + "/" + task
	out := ROUT + task
	return c.task(c.reducerbin, []string{in, out, strconv.Itoa(c.nmaptask)})
}

func (c *Coord) claimEntry(dir string, st *np.Stat) (string, error) {
	from := dir + "/" + st.Name
	if err := c.Rename(from, dir+TIP+"/"+st.Name); err != nil {
		if np.IsErrUnreachable(err) { // partitioned?  (XXX other errors than EOF?)
			return "", err
		}
		// another coord thread claimed the task before us
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

type Ttask struct {
	task   string
	status *proc.Status
	err    error
}

func (c *Coord) startTasks(dir string, ch chan Ttask, f func(string) (*proc.Status, error)) int {
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
		go func() {
			db.DPrintf("MR", "start task %v\n", t)
			status, err := f(t)
			ch <- Ttask{t, status, err}
		}()
	}
	return n
}

func (c *Coord) restartMappers(files []string) {
	db.DPrintf(db.ALWAYS, "restart mappers %v\n", files)
	for _, f := range files {
		n := path.Join(MDIR, f)
		if _, err := c.PutFile(n, 0777, np.OWRITE, []byte(n)); err != nil {
			db.DFatalf("PutFile %v err %v\n", n, err)
		}
	}
}

func (c *Coord) processResult(dir string, res Ttask) {
	if res.err == nil && res.status.IsStatusOK() {
		// mark task as done
		db.DPrintf(db.ALWAYS, "task done %v\n", res.task)
		s := dir + TIP + "/" + res.task
		d := dir + DONE + "/" + res.task
		err := c.Rename(s, d)
		if err != nil {
			// an earlier instance already succeeded
			log.Printf("%v: rename %v to %v err %v\n", proc.GetName(), s, d, err)
		}
	} else {
		// task failed; make it runnable again
		to := dir + "/" + res.task
		db.DPrintf("MR", "task %v failed %v err %v\n", res.task, res.status, res.err)
		if err := c.Rename(dir+TIP+"/"+res.task, to); err != nil {
			db.DFatalf("%v: rename to %v err %v\n", proc.GetName(), to, err)
		}
	}
}

func (c *Coord) stragglers(dir string, ch chan Ttask, f func(string) (*proc.Status, error)) {
	sts, err := c.GetDir(dir + TIP) // XXX handle one entry at the time?
	if err != nil {
		db.DFatalf("stragglers ReadDir %v err %v\n", dir+TIP, err)
	}
	n := 0
	for _, st := range sts {
		n += 1
		go func() {
			db.DPrintf("start straggler task %v\n", st.Name)
			status, err := f(st.Name)
			ch <- Ttask{st.Name, status, err}
		}()
	}
}

func (c *Coord) recover(dir string) {
	sts, err := c.GetDir(dir + TIP) // XXX handle one entry at the time?
	if err != nil {
		db.DFatalf("recover: ReadDir %v err %v\n", dir+TIP, err)
	}

	// just treat all tasks in progress as failed; too aggressive, but correct.
	for _, st := range sts {
		db.DPrintf(db.ALWAYS, "recover %v\n", st.Name)
		to := dir + "/" + st.Name
		if c.Rename(dir+TIP+"/"+st.Name, to) != nil {
			// an old, disconnected coord may do this too,
			// if one of its tasks fails
			log.Printf("%v: rename to %v err %v\n", proc.GetName(), to, err)
		}
	}
}

func (c *Coord) phase(dir string, f func(string) (*proc.Status, error)) bool {
	start := time.Now()
	ch := make(chan Ttask)
	//	straggler := false
	for n := c.startTasks(dir, ch, f); n > 0; n-- {
		res := <-ch
		c.processResult(dir, res)
		if !res.status.IsStatusOK() {
			// If we're reducing and can't find some mapper output, a ux may have
			// crashed. So, restart those map tasks.
			if dir == RDIR && res.status.Msg() == RESTART {
				log.Printf("data %v\n", res.status.Data())
				lostMappers := res.status.Data().([]string)
				c.restartMappers(lostMappers)
				return false
			} else {
				n += c.startTasks(dir, ch, f)
			}
		}
		//		if n == 2 && !straggler { // XXX percentage of total computation
		//			straggler = true
		//			c.stragglers(dir, ch, f)
		//		}
	}
	db.DPrintf(db.ALWAYS, "Phase %v %v\n", dir, time.Since(start).Milliseconds())
	return true
}

func (c *Coord) Work() {
	// Try to become the leading coordinator.  If we get
	// partitioned, we cannot write the todo directories either.
	c.electclnt.AcquireLeadership(nil)

	db.DPrintf(db.ALWAYS, "leader\n")

	for {
		c.recover(MDIR)
		c.recover(RDIR)
		c.phase(MDIR, c.mapper)

		// If reduce phase is unsuccessful, we lost some
		// mapper output. Restart those mappers.
		success := c.phase(RDIR, c.reducer)
		if success {
			break
		}
		log.Printf("try again\n")
	}

	c.Exited(proc.MakeStatus(proc.StatusOK))
}
