package mr

import (
	"errors"
	"log"
	"strconv"

	// db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

const (
	INPUTDIR = "name/s3/~ip/input/"
	MRDIR    = "name/mr"
	MDIR     = "name/mr/m"
	RDIR     = "name/mr/r"
	ROUT     = "name/mr/mr-out-"
	CLAIMED  = "-claimed"
	DONE     = "-done"
)

func InitWorkerFS(fsl *fslib.FsLib) {
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

	// input directories for reduce tasks
	for r := 0; r < NReduce; r++ {
		n := "name/mr/r/" + strconv.Itoa(r)
		if err := fsl.Mkdir(n, 0777); err != nil {
			log.Fatalf("Mkdir %v err %v\n", n, err)
		}
	}
}

type Worker struct {
	*fslib.FsLib
	proc.ProcClnt
	mapperbin  string
	reducerbin string
}

func MakeWorker(args []string) (*Worker, error) {
	if len(args) != 2 {
		return nil, errors.New("MakeWorker: too few arguments")
	}
	log.Printf("MakeWorker %v\n", args)
	w := &Worker{}
	w.FsLib = fslib.MakeFsLib("worker-" + procinit.GetPid())
	w.mapperbin = args[0]
	w.reducerbin = args[1]
	w.ProcClnt = procinit.MakeProcClnt(w.FsLib, procinit.GetProcLayersMap())
	w.Started(procinit.GetPid())
	return w, nil
}

func (w *Worker) mapper(task string) string {
	log.Printf("task: %v\n", task)
	pid := proc.GenPid()
	task = INPUTDIR + task
	a := proc.MakeProc(pid, w.mapperbin, []string{task})
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

func (w *Worker) getTask(dir string) (string, error) {
	sts, err := w.ReadDir(dir)
	if err != nil {
		log.Printf("Readdir getTask %v err %v\n", dir, err)
		return "", err
	}
	for _, st := range sts {
		log.Printf("try to claim %v\n", st.Name)
		_, err := w.PutFile(dir+CLAIMED+"/"+st.Name, []byte{}, 0777|np.DMTMP, np.OWRITE)
		if err == nil {
			return st.Name, nil
		}
	}
	return "", nil
}

func (w *Worker) doWork(dir string, f func(string) string) error {
	for {
		t, err := w.getTask(dir)
		if err != nil {
			return err
		}
		if t == "" {
			break
		}
		ok := f(t)
		log.Printf("task %v returned %v\n", t, ok)
	}
	return nil
}

func (w *Worker) Work() {
	err := w.doWork(MDIR, w.mapper)
	if err == nil {
		err = w.doWork(RDIR, w.reducer)
		if err == nil {
			w.Exited(procinit.GetPid(), "OK")
			return
		}
	}
	w.Exited(procinit.GetPid(), err.Error())
}
