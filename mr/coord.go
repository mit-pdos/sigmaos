package mr

import (
	"errors"
	"fmt"
	"log"
	// "math/rand"
	"strconv"
	// "time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
	usync "ulambda/sync"
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
	proc.ProcClnt
	crashCoord  string
	nreducetask int
	crash       string
	mapperbin   string
	reducerbin  string
	lock        *usync.Lock
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) != 5 {
		return nil, errors.New("MakeCoord: too few arguments")
	}
	log.Printf("MakeCoord %v\n", args)
	w := &Coord{}
	w.FsLib = fslib.MakeFsLib("coord-" + proc.GetPid())

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("MakeCoord: nreducetask %v isn't int", args[1])
	}

	w.nreducetask = n
	w.mapperbin = args[1]
	w.reducerbin = args[2]
	w.crash = args[3]
	w.crashCoord = args[4]

	w.ProcClnt = procinit.MakeProcClnt(w.FsLib, procinit.GetProcLayersMap())

	w.lock = usync.MakeLock(w.FsLib, MRDIR, "lock-coord", true)

	w.Started(proc.GetPid())

	return w, nil
}

// // XXX set timer based on # of workers?
// func (w *Coord) monitor() {
// 	for true {
// 		ms := 1000 + (rand.Int63() % 10000)
// 		time.Sleep(time.Duration(ms) * time.Millisecond)
// 		log.Printf("%v: signal\n", db.GetName())
// 		// wakeup one worker to check
// 		w.cond.Signal()
// 	}
// }

func (w *Coord) mapper(task string) string {
	pid := proc.GenPid()
	input := INPUTDIR + task
	a := proc.MakeProc(pid, w.mapperbin, []string{w.crash, strconv.Itoa(w.nreducetask), input})
	w.Spawn(a)
	ok, err := w.WaitExit(pid)
	if err != nil {
		return err.Error()
	}
	return ok
}

func (w *Coord) reducer(task string) string {
	pid := proc.GenPid()
	in := RDIR + TIP + "/" + task
	out := ROUT + task
	a := proc.MakeProc(pid, w.reducerbin, []string{w.crash, in, out})
	w.Spawn(a)
	ok, err := w.WaitExit(pid)
	if err != nil {
		return err.Error()
	}
	return ok
}

func (w *Coord) claimEntry(dir string, st *np.Stat) string {
	from := dir + "/" + st.Name
	if err := w.Rename(from, dir+TIP+"/"+st.Name); err != nil {
		// another coord claimed the task
		return ""
	}
	return st.Name
}

func (w *Coord) getTask(dir string) string {
	t := ""
	stopped, err := w.ProcessDir(dir, func(st *np.Stat) (bool, error) {
		t = w.claimEntry(dir, st)
		if t == "" {
			return false, nil
		}
		return true, nil

	})
	if err != nil {
		log.Fatalf("Readdir getTask %v err %v\n", dir, err)
	}
	if stopped {
		return t
	}
	return ""
}

type Ttask struct {
	task string
	ok   string
}

func (w *Coord) startTasks(dir string, ch chan Ttask, f func(string) string) int {
	n := 0
	for {
		t := w.getTask(dir)
		if t == "" {
			break
		}
		n += 1
		go func() {
			log.Printf("%v: start task %v\n", db.GetName(), t)
			ok := f(t)
			ch <- Ttask{t, ok}
		}()
	}
	return n
}

func (w *Coord) processResult(dir string, res Ttask) {
	if res.ok == "OK" {
		// mark task as done

		log.Printf("%v: task done %v\n", db.GetName(), res.task)
		f := dir + DONE + "/" + res.task
		_, err := w.PutFile(f, []byte{}, 0777, np.OWRITE)
		if err != nil {
			log.Fatalf("getTask: putfile %v err %v\n", f, err)
		}

		// Remove from in-progress tasks
		f = dir + TIP + "/" + res.task
		if w.Remove(f) != nil {
			log.Fatalf("getTask: remove %v err %v\n", f, err)
		}
	} else {
		// task failed; make it runnable again

		to := dir + "/" + res.task
		log.Printf("%v: task %v failed %v\n", db.GetName(), res.task, res.ok)
		if err := w.Rename(dir+TIP+"/"+res.task, to); err != nil {
			log.Fatalf("doWork: rename to %v err %v\n", to, err)
		}
	}
}

func (w *Coord) recover(dir string) {
	sts, err := w.ReadDir(dir + TIP) // handle one entry at the time?
	if err != nil {
		log.Fatalf("recover: ReadDir %v err %v\n", dir+TIP, err)
	}

	if len(sts) > 0 {
		// don't crash the backup
		w.crashCoord = "NO"
	}

	// just treat all tasks in progress as failed; too aggressive, but correct.
	for _, st := range sts {
		log.Printf("%v: recover %v\n", db.GetName(), st.Name)
		to := dir + "/" + st.Name
		if w.Rename(dir+TIP+"/"+st.Name, to) != nil {
			// an old, disconnected coord may do this too,
			// if one of its tasks fails
			log.Printf("recover: rename to %v err %v\n", to, err)
		}
	}
}

func (w *Coord) phase(dir string, f func(string) string) {
	ch := make(chan Ttask)
	for n := w.startTasks(dir, ch, f); n > 0; n-- {
		res := <-ch
		if w.crashCoord == "YES" {
			MaybeCrash()
		}
		w.processResult(dir, res)
		if res.ok != "OK" {
			n += w.startTasks(dir, ch, f)
		}
	}
	// XXX double check no tasks are in TIP because of disconnected old coord
}

func (w *Coord) Work() {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.recover(MDIR)
	w.recover(RDIR)

	w.phase(MDIR, w.mapper)
	log.Printf("%v: Reduce phase\n", db.GetName())
	w.phase(RDIR, w.reducer)

	w.Exited(proc.GetPid(), "OK")
}
