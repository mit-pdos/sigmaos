package mr

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

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

	lock := usync.MakeLock(fsl, MRDIR, "lock-done", true)
	cond := usync.MakeCondNew(fsl, MRDIR, "cond-done", lock)

	cond.Init()
}

type Coord struct {
	*fslib.FsLib
	proc.ProcClnt
	nreducetask int
	crash       string
	mapperbin   string
	reducerbin  string
	lock        *usync.Lock
	cond        *usync.Cond
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) != 4 {
		return nil, errors.New("MakeCoord: too few arguments")
	}
	log.Printf("MakeCoord %v\n", args)
	w := &Coord{}
	w.FsLib = fslib.MakeFsLib("coord-" + procinit.GetPid())
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("MakeCoord: nreducetask %v isn't int", args[0])
	}
	w.nreducetask = n
	w.mapperbin = args[1]
	w.reducerbin = args[2]
	w.crash = args[3]

	w.ProcClnt = procinit.MakeProcClnt(w.FsLib, procinit.GetProcLayersMap())

	w.lock = usync.MakeLock(w.FsLib, MRDIR, "lock-done", true)
	w.cond = usync.MakeCondNew(w.FsLib, MRDIR, "cond-done", w.lock)

	// go w.monitor()

	w.Started(procinit.GetPid())

	return w, nil
}

// XXX set timer based on # of workers?
func (w *Coord) monitor() {
	for true {
		ms := 1000 + (rand.Int63() % 10000)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		log.Printf("%v: signal\n", db.GetName())
		// wakeup one worker to check
		w.cond.Signal()
	}
}

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
		// log.Printf("claimEntry: rename from %v err %v\n", from, err)
		// some other worker claimed the task
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
		log.Printf("%v: task done %v\n", db.GetName(), res.task)
		// Mark task as done
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

// XXX look one at time?
func (w *Coord) redoWork(dir string) bool {
	sts, err := w.ReadDir(dir + TIP)
	if err != nil {
		log.Fatalf("redoWork: ReadDir %v err %v\n", dir+TIP, err)
	}
	for _, st := range sts {
		if false {
			log.Printf("%v: redo?: %v\n", db.GetName(), st.Name)
			to := dir + "/" + st.Name
			if w.Rename(dir+TIP+"/"+st.Name, to) != nil {
				log.Fatalf("redoWork: rename to %v err %v\n", to, err)
			}
			return true
		}
	}
	return false
}

// XXX read dir incrementally
func (w *Coord) barrier(dir string) bool {
	w.lock.Lock()
	for {
		sts, err := w.ReadDir(dir + TIP)
		if err != nil {
			log.Fatalf("Readdir waitForMappers %v err %v\n", dir+TIP, err)
		}
		if len(sts) == 0 {
			log.Printf("%v: barrier done\n", dir)
			break
		}
		log.Printf("wait dir %v %v e.g. %v\n", dir, len(sts), sts[0].Name)
		w.cond.Wait()
		//if w.redoWork(dir) {
		//	return false
		//}
	}
	w.lock.Unlock()
	return true
}

func (w *Coord) phase(dir string, f func(string) string) {
	ch := make(chan Ttask)
	for n := w.startTasks(dir, ch, f); n > 0; n-- {
		res := <-ch
		w.processResult(dir, res)
		if res.ok != "OK" {
			n += w.startTasks(dir, ch, f)
		}
	}
}

func (w *Coord) Work() {
	w.phase(MDIR, w.mapper)
	log.Printf("%v: Reduce phase\n", db.GetName())
	w.phase(RDIR, w.reducer)
	w.Exited(procinit.GetPid(), "OK")
}
