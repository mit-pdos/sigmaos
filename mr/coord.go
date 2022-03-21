package mr

import (
	"errors"
	"fmt"
	"log"
	"path"
	"strconv"

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
	MDIR     = "name/mr/m"
	RDIR     = "name/mr/r"
	ROUT     = "name/mr/mr-out-"
	CLAIMED  = "-claimed"
	TIP      = "-tip"
	DONE     = "-done"
	RESTART  = "restart"
	NCOORD   = 3
)

func InitCoordFS(fsl *fslib.FsLib, nreducetask int) {
	for _, n := range []string{MRDIR, MDIR, RDIR, MDIR + CLAIMED, RDIR + CLAIMED, MDIR + TIP, RDIR + TIP, MDIR + DONE, RDIR + DONE} {
		if err := fsl.MkDir(n, 0777); err != nil {
			log.Fatalf("Mkdir %v\n", err)
		}
	}

	// input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := RDIR + "/" + strconv.Itoa(r)
		if err := fsl.MkDir(n, 0777); err != nil {
			log.Fatalf("Mkdir %v err %v\n", n, err)
		}
	}
}

type Coord struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	nreducetask int
	crash       int
	mapperbin   string
	reducerbin  string
	electclnt   *electclnt.ElectClnt
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) != 4 {
		return nil, errors.New("MakeCoord: too few arguments")
	}
	w := &Coord{}
	w.FsLib = fslib.MakeFsLib("coord-" + proc.GetPid().String())

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("MakeCoord: nreducetask %v isn't int", args[1])
	}

	w.nreducetask = n
	w.mapperbin = args[1]
	w.reducerbin = args[2]

	c, err := strconv.Atoi(args[3])
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
	return c.task(c.reducerbin, []string{in, out})
}

func (c *Coord) claimEntry(dir string, st *np.Stat) (string, error) {
	from := dir + "/" + st.Name
	if err := c.Rename(from, dir+TIP+"/"+st.Name); err != nil {
		if np.IsErrUnreachable(err) { // partitioned?  (XXX other errors than EOF?)
			return "", err
		}
		// another coord claimed the task
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
			log.Fatalf("getTask %v err %v\n", dir, err)
		}
		if t == "" {
			break
		}
		n += 1
		go func() {
			db.DLPrintf("MR", "start task %v\n", t)
			status, err := f(t)
			ch <- Ttask{t, status, err}
		}()
	}
	return n
}

func (c *Coord) restartMappers(files []string) {
	for _, f := range files {
		n := path.Join(MDIR, f)
		if _, err := c.PutFile(n, 0777, np.OWRITE, []byte(n)); err != nil {
			log.Fatalf("PutFile %v err %v\n", n, err)
		}
	}
}

func (c *Coord) processResult(dir string, res Ttask) {
	if res.err == nil && res.status.IsStatusOK() {
		// mark task as done
		log.Printf("%v: task done %v\n", proc.GetName(), res.task)
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
		db.DLPrintf("MR", "task %v failed %v err %v\n", res.task, res.status, res.err)
		if err := c.Rename(dir+TIP+"/"+res.task, to); err != nil {
			log.Fatalf("%v: rename to %v err %v\n", proc.GetName(), to, err)
		}
	}
}

func (c *Coord) stragglers(dir string, ch chan Ttask, f func(string) (*proc.Status, error)) {
	sts, err := c.GetDir(dir + TIP) // XXX handle one entry at the time?
	if err != nil {
		log.Fatalf("%v: FATAL stragglers ReadDir %v err %v\n", proc.GetName(), dir+TIP, err)
	}
	n := 0
	for _, st := range sts {
		n += 1
		go func() {
			db.DLPrintf("start straggler task %v\n", st.Name)
			status, err := f(st.Name)
			ch <- Ttask{st.Name, status, err}
		}()
	}
}

func (c *Coord) recover(dir string) {
	sts, err := c.GetDir(dir + TIP) // XXX handle one entry at the time?
	if err != nil {
		log.Fatalf("%v: FATAL recover: ReadDir %v err %v\n", proc.GetName(), dir+TIP, err)
	}

	// just treat all tasks in progress as failed; too aggressive, but correct.
	for _, st := range sts {
		log.Printf("%v: recover %v\n", proc.GetName(), st.Name)
		to := dir + "/" + st.Name
		if c.Rename(dir+TIP+"/"+st.Name, to) != nil {
			// an old, disconnected coord may do this too,
			// if one of its tasks fails
			log.Printf("%v: rename to %v err %v\n", proc.GetName(), to, err)
		}
	}
}

func (c *Coord) phase(dir string, f func(string) (*proc.Status, error)) bool {
	ch := make(chan Ttask)
	straggler := false
	for n := c.startTasks(dir, ch, f); n > 0; n-- {
		res := <-ch
		c.processResult(dir, res)
		if !res.status.IsStatusOK() {
			// If we're reducing and can't find some mapper output, a ux may have
			// crashed. So, restart those map tasks.
			if dir == RDIR && res.status.Msg() == RESTART {
				lostMappers := res.status.Data().([]string)
				c.restartMappers(lostMappers)
				return false
			} else {
				n += c.startTasks(dir, ch, f)
			}
		}
		if n == 2 && !straggler { // XXX percentage of total computation
			straggler = true
			c.stragglers(dir, ch, f)
		}
	}
	return true
}

func (c *Coord) Work() {
	// Try to become the leading coordinator.  If we get
	// partitioned, we cannot write the todo directories either.
	c.electclnt.AcquireLeadership(nil)

	log.Printf("%v: leader\n", proc.GetName())

	for {
		c.recover(MDIR)
		c.recover(RDIR)
		c.phase(MDIR, c.mapper)
		log.Printf("%v: Reduce phase\n", proc.GetName())
		// If reduce phase is unsuccessful, we lost some mapper output. Restart
		// those mappers.
		success := c.phase(RDIR, c.reducer)
		if success {
			break
		}
	}

	c.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
}
