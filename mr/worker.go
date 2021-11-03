package mr

import (
	"errors"
	"log"

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
	MCLAIM   = "name/mr/m-claimed"
	RCLAIM   = "name/mr/r-claimed"
	ROUT     = "name/mr/mr-out-"
)

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

func (w *Worker) doMapper() error {
	isWork := true
	for isWork {
		sts, err := w.ReadDir(MDIR)
		if err != nil {
			log.Printf("Readdir mapper err %v\n", err)
			return err
		}
		isWork = false
		for _, st := range sts {
			log.Printf("try to claim %v\n", st.Name)
			_, err := w.PutFile(MCLAIM+"/"+st.Name, []byte{}, 0777|np.DMTMP, np.OWRITE)
			if err == nil {
				ok := w.mapper(st.Name)
				log.Printf("task returned %v\n", ok)
				isWork = true
			}
		}
	}
	return nil
}

func (w *Worker) doReducer() error {
	isWork := true
	for isWork {
		sts, err := w.ReadDir(RDIR)
		if err != nil {
			log.Printf("Readdir reducer err %v\n", err)
			return err
		}
		isWork = false
		n := 0
		for _, st := range sts {
			log.Printf("try to claim %v\n", st.Name)
			_, err := w.PutFile(RCLAIM+"/"+st.Name, []byte{}, 0777|np.DMTMP, np.OWRITE)
			if err == nil {
				ok := w.reducer(st.Name)
				log.Printf("task returned %v\n", ok)
				isWork = true
			}
			n += 1
		}
	}
	return nil
}

func (w *Worker) Work() {
	err := w.doMapper()
	if err == nil {
		err = w.doReducer()
		if err == nil {
			w.Exited(procinit.GetPid(), "OK")
			return
		}
	}
	w.Exited(procinit.GetPid(), err.Error())
}
