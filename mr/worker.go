package mr

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	// db "ulambda/debug"
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
	if err := fsl.Mkdir(MRDIR, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(MDIR, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(RDIR, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(MDIR+CLAIMED, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(RDIR+CLAIMED, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(MDIR+TIP, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(RDIR+TIP, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(MDIR+DONE, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(RDIR+DONE, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	lock := usync.MakeLock(fsl, MRDIR, "lock-done", true)
	cond := usync.MakeCondNew(fsl, MRDIR, "cond-done", lock)

	cond.Init()

	// input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := "name/mr/r/" + strconv.Itoa(r)
		if err := fsl.Mkdir(n, 0777); err != nil {
			log.Fatalf("Mkdir %v err %v\n", n, err)
		}
	}
}

type Worker struct {
	*fslib.FsLib
	proc.ProcClnt
	nreducetask int
	mapperbin   string
	reducerbin  string
	lock        *usync.Lock
	cond        *usync.Cond
}

func MakeWorker(args []string) (*Worker, error) {
	if len(args) != 3 {
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

	w.ProcClnt = procinit.MakeProcClnt(w.FsLib, procinit.GetProcLayersMap())

	w.lock = usync.MakeLock(w.FsLib, MRDIR, "lock-done", true)
	w.cond = usync.MakeCondNew(w.FsLib, MRDIR, "cond-done", w.lock)

	w.Started(procinit.GetPid())
	return w, nil
}

func (w *Worker) mapper(task string) string {
	log.Printf("task: %v\n", task)

	// Spawn task
	pid := proc.GenPid()
	input := INPUTDIR + task
	a := proc.MakeProc(pid, w.mapperbin, []string{strconv.Itoa(w.nreducetask), input})
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
	in := RDIR + "/" + task
	out := ROUT + task
	a := proc.MakeProc(pid, w.reducerbin, []string{in, out})
	w.Spawn(a)
	ok, err := w.WaitExit(pid)
	if err != nil {
		return err.Error()
	}
	return ok
}

func (w *Worker) processEntry(dir string, st *np.Stat) string {
	log.Printf("try to claim %v\n", st.Name)
	if _, err := w.PutFile(dir+CLAIMED+"/"+st.Name, []byte{}, 0777|np.DMTMP, np.OWRITE); err == nil {
		from := dir + "/" + st.Name
		if w.Rename(from, dir+TIP+"/"+st.Name) != nil {
			log.Fatalf("getTask: rename %v err %v\n", from, err)
		}
		return st.Name
	}
	// some other worker claimed the task
	return ""
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
	}
}

func (w *Worker) barrier(dir string) {
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
	}
	w.lock.Unlock()
}

func (w *Worker) Work() {
	w.doWork(MDIR, w.mapper)
	w.barrier(MDIR)
	w.doWork(RDIR, w.reducer)
	w.barrier(RDIR)
	w.Exited(procinit.GetPid(), "OK")
}
