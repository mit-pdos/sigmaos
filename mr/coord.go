package mr

import (
	"errors"
	"fmt"
	"log"
	"path"
	"strconv"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fenceclnt"
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
		if err := fsl.Mkdir(n, 0777); err != nil {
			log.Fatalf("Mkdir %v\n", err)
		}
	}

	// input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := RDIR + "/" + strconv.Itoa(r)
		if err := fsl.Mkdir(n, 0777); err != nil {
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
	fence       *fenceclnt.FenceClnt
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) != 4 {
		return nil, errors.New("MakeCoord: too few arguments")
	}
	w := &Coord{}
	w.FsLib = fslib.MakeFsLib("coord-" + proc.GetPid())

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

	w.Started(proc.GetPid())

	w.fence = fenceclnt.MakeFenceClnt(w.FsLib, MRDIR+"/fence-coord", 0, []string{MRDIR})

	crash.Crasher(w.FsLib)

	return w, nil
}

func (w *Coord) task(bin string, args []string) (*proc.Status, error) {
	a := proc.MakeProc(bin, args)
	if w.crash > 0 {
		a.AppendEnv("SIGMACRASH", strconv.Itoa(w.crash))
	}
	err := w.Spawn(a)
	if err != nil {
		return nil, err
	}
	status, err := w.WaitExit(a.Pid)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (w *Coord) mapper(task string) (*proc.Status, error) {
	input := INPUTDIR + task
	return w.task(w.mapperbin, []string{strconv.Itoa(w.nreducetask), input})
}

func (w *Coord) reducer(task string) (*proc.Status, error) {
	in := RDIR + TIP + "/" + task
	out := ROUT + task
	return w.task(w.reducerbin, []string{in, out})
}

func (w *Coord) claimEntry(dir string, st *np.Stat) (string, error) {
	from := dir + "/" + st.Name
	if err := w.Rename(from, dir+TIP+"/"+st.Name); err != nil {
		if np.IsErrEOF(err) { // partitioned?  (XXX other errors than EOF?)
			return "", err
		}
		// another coord claimed the task
		return "", nil
	}
	return st.Name, nil
}

func (w *Coord) getTask(dir string) (string, error) {
	t := ""
	stopped, err := w.ProcessDir(dir, func(st *np.Stat) (bool, error) {
		t1, err := w.claimEntry(dir, st)
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

func (w *Coord) startTasks(dir string, ch chan Ttask, f func(string) (*proc.Status, error)) int {
	n := 0
	for {
		t, err := w.getTask(dir)
		if err != nil {
			log.Fatalf("getTask %v err %v\n", dir, err)
		}
		if t == "" {
			break
		}
		n += 1
		go func() {
			db.DPrintf("start task %v\n", t)
			status, err := f(t)
			ch <- Ttask{t, status, err}
		}()
	}
	return n
}

func (w *Coord) restartMappers(files []string) {
	for _, f := range files {
		n := path.Join(MDIR, f)
		if _, err := w.PutFile(n, 0777, np.OWRITE, []byte(n)); err != nil {
			log.Fatalf("PutFile %v err %v\n", n, err)
		}
	}
}

func (w *Coord) processResult(dir string, res Ttask) {
	if res.err == nil && res.status.IsStatusOK() {
		// mark task as done
		log.Printf("%v: task done %v\n", proc.GetProgram(), res.task)
		s := dir + TIP + "/" + res.task
		d := dir + DONE + "/" + res.task
		err := w.Rename(s, d)
		if err != nil {
			// an earlier instance already succeeded
			log.Printf("%v: rename %v to %v err %v\n", proc.GetProgram(), s, d, err)
		}
	} else {
		// task failed; make it runnable again
		to := dir + "/" + res.task
		db.DPrintf("task %v failed %v err %v\n", res.task, res.status, res.err)
		if err := w.Rename(dir+TIP+"/"+res.task, to); err != nil {
			log.Fatalf("%v: rename to %v err %v\n", proc.GetProgram(), to, err)
		}
	}
}

func (w *Coord) stragglers(dir string, ch chan Ttask, f func(string) (*proc.Status, error)) {
	sts, err := w.GetDir(dir + TIP) // XXX handle one entry at the time?
	if err != nil {
		log.Fatalf("recover: ReadDir %v err %v\n", dir+TIP, err)
	}
	n := 0
	for _, st := range sts {
		n += 1
		go func() {
			log.Printf("%v: start straggler task %v\n", proc.GetProgram(), st.Name)
			status, err := f(st.Name)
			ch <- Ttask{st.Name, status, err}
		}()
	}
}

func (w *Coord) recover(dir string) {
	sts, err := w.GetDir(dir + TIP) // XXX handle one entry at the time?
	if err != nil {
		log.Fatalf("recover: ReadDir %v err %v\n", dir+TIP, err)
	}

	// just treat all tasks in progress as failed; too aggressive, but correct.
	for _, st := range sts {
		log.Printf("%v: recover %v\n", proc.GetProgram(), st.Name)
		to := dir + "/" + st.Name
		if w.Rename(dir+TIP+"/"+st.Name, to) != nil {
			// an old, disconnected coord may do this too,
			// if one of its tasks fails
			log.Printf("%v: rename to %v err %v\n", proc.GetProgram(), to, err)
		}
	}
}

func (w *Coord) phase(dir string, f func(string) (*proc.Status, error)) bool {
	ch := make(chan Ttask)
	straggler := false
	for n := w.startTasks(dir, ch, f); n > 0; n-- {
		res := <-ch
		w.processResult(dir, res)
		if !res.status.IsStatusOK() {
			// If we're reducing and can't find some mapper output, a ux may have
			// crashed. So, restart those map tasks.
			if dir == RDIR && res.status.Msg() == RESTART {
				lostMappers := res.status.Data().([]string)
				w.restartMappers(lostMappers)
				return false
			} else {
				n += w.startTasks(dir, ch, f)
			}
		}
		if n == 2 && !straggler { // XXX percentage of total computation
			straggler = true
			w.stragglers(dir, ch, f)
		}
	}
	return true
}

func (w *Coord) Work() {
	// Try to become the primary coordinator.  Backup coordinators
	// will be able to acquire the fence if the primary fails or
	// is partitioned.
	w.fence.AcquireFenceW([]byte{})
	defer w.fence.ReleaseFence()

	log.Printf("%v: primary\n", proc.GetProgram())

	for {
		w.recover(MDIR)
		w.recover(RDIR)
		w.phase(MDIR, w.mapper)
		log.Printf("%v: Reduce phase\n", proc.GetProgram())
		// If reduce phase is unsuccessful, we lost some mapper output. Restart
		// those mappers.
		success := w.phase(RDIR, w.reducer)
		if success {
			break
		}
	}

	w.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
}
