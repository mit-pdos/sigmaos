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

func InitWorkerFS(fsl *fslib.FsLib, nreducetask int) {
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

type Worker struct {
	*fslib.FsLib
	proc.ProcClnt
	nreducetask int
	crash       string
	mapperbin   string
	reducerbin  string
	lock        *usync.Lock
	cond        *usync.Cond
}

func MakeWorker(args []string) (*Worker, error) {
	if len(args) != 4 {
		return nil, errors.New("MakeWorker: too few arguments")
	}
	log.Printf("MakeWorker %v\n", args)
	w := &Worker{}
	w.FsLib = fslib.MakeFsLib("worker-" + procinit.GetPid())
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("MakeWorker: nreducetask %v isn't int", args[0])
	}
	w.nreducetask = n
	w.mapperbin = args[1]
	w.reducerbin = args[2]
	w.crash = args[3]

	w.ProcClnt = procinit.MakeProcClnt(w.FsLib, procinit.GetProcLayersMap())

	w.lock = usync.MakeLock(w.FsLib, MRDIR, "lock-done", true)
	w.cond = usync.MakeCondNew(w.FsLib, MRDIR, "cond-done", w.lock)

	go w.monitor()

	w.Started(procinit.GetPid())

	return w, nil
}

// XXX set timer based on # of workers?
func (w *Worker) monitor() {
	for true {
		ms := 1000 + (rand.Int63() % 10000)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		log.Printf("%v: signal\n", db.GetName())
		// wakeup one worker to check
		w.cond.Signal()
	}
}

func (w *Worker) mapper(task string) string {
	log.Printf("task: %v\n", task)

	// Spawn task
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

func (w *Worker) reducer(task string) string {
	log.Printf("task: %v\n", task)

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

func (w *Worker) processEntry(dir string, st *np.Stat) string {
	log.Printf("try to claim %v\n", st.Name)

	from := dir + "/" + st.Name
	if err := w.Rename(from, dir+TIP+"/"+st.Name); err != nil {
		log.Printf("getTask: rename from %v err %v\n", from, err)
		// some other worker claimed the task
		return ""
	}
	return st.Name
}

func (w *Worker) getTask(dir string) string {
	t := ""
	stopped, err := w.ProcessDir(dir, func(st *np.Stat) (bool, error) {
		t = w.processEntry(dir, st)
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

func (w *Worker) doWork(dir string, f func(string) string) {
	for {
		t := w.getTask(dir)
		if t == "" {
			break
		}
		ok := f(t)
		log.Printf("task %v returned %v\n", t, ok)
		if ok == "OK" {

			// Mark task as done
			f := dir + DONE + "/" + t
			_, err := w.PutFile(f, []byte{}, 0777, np.OWRITE)
			if err != nil {
				log.Fatalf("getTask: putfile %v err %v\n", f, err)
			}

			// Remove from in-progress tasks
			f = dir + TIP + "/" + t
			if w.Remove(f) != nil {
				log.Fatalf("getTask: remove %v err %v\n", f, err)
			}

			// Signal waiters
			w.cond.Broadcast()
		} else {
			// task failed; make it runnable again
			to := dir + "/" + t
			if err := w.Rename(dir+TIP+"/"+t, to); err != nil {
				log.Fatalf("doWork: rename to %v err %v\n", to, err)
			}
		}
	}
}

func (w *Worker) redoWork(dir string) bool {
	sts, err := w.ReadDir(dir + TIP)
	if err != nil {
		log.Fatalf("redoWork: ReadDir %v err %v\n", dir+TIP, err)
	}
	for _, st := range sts {
		log.Printf("%v: redo?: %v\n", db.GetName(), st.Name)
		if false { // XXX look at time?
			to := dir + "/" + st.Name
			if w.Rename(dir+TIP+"/"+st.Name, to) != nil {
				log.Fatalf("redoWork: rename to %v err %v\n", to, err)
			}
			return true
		} else {
			log.Printf("still claimed %v\n", st)
		}
	}
	return false
}

// XXX read dir incrementally
func (w *Worker) barrier(dir string) bool {
	w.lock.Lock()
	for {
		sts, err := w.ReadDir(dir + TIP)
		if err != nil {
			log.Fatalf("Readdir waitForMappers %v err %v\n", dir+TIP, err)
		}
		if len(sts) == 0 {
			break
		}
		log.Printf("wait %v\n", sts)
		w.cond.Wait()
		if w.redoWork(dir) {
			return false
		}
	}
	w.lock.Unlock()
	return true
}

func (w *Worker) Work() {
	for true {
		w.doWork(MDIR, w.mapper)
		if w.barrier(MDIR) {
			break
		}
	}
	w.doWork(RDIR, w.reducer)
	w.barrier(RDIR)
	w.Exited(procinit.GetPid(), "OK")
}
