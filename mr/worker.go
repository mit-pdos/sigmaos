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

type Worker struct {
	*fslib.FsLib
	proc.ProcClnt
	mapperbin  string
	reducerbin string
}

// XXX create in a temporary file and then rename
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

	//err := w.Mkdir("name/ux/~ip/m-"+w.output, 0777)
	//if err != nil {
	//	return nil, fmt.Errorf("Makemapper: cannot create %v: %v\n", w.output, err)
	//}

	w.Started(procinit.GetPid())
	return w, nil
}

func (w *Worker) mapper(fn string, n string) string {
	log.Printf("task: %v\n", fn)
	pid := proc.GenPid()
	fn = "name/s3/~ip/input/" + fn
	a := proc.MakeProc(pid, w.mapperbin, []string{fn, n})
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
		sts, err := w.ReadDir("name/mr/m")
		if err != nil {
			log.Printf("Readdir mapper err %v\n", err)
			return err
		}
		isWork = false
		// XXX use file name for claim
		n := 0
		for _, st := range sts {
			log.Printf("try to claim %v\n", st.Name)
			_, err := w.PutFile("name/mr/m-claimed/"+st.Name, []byte{}, 0777|np.DMTMP, np.OWRITE)
			if err == nil {
				ok := w.mapper(st.Name, strconv.Itoa(n))
				log.Printf("task returned %v\n", ok)
				isWork = true
			}
			n += 1
		}
	}
	return nil
}

func (w *Worker) reducer(job string) string {
	log.Printf("task: %v\n", job)
	pid := proc.GenPid()
	in := "name/mr/r/" + job
	out := "name/mr/mr-out-" + job
	a := proc.MakeProc(pid, w.reducerbin, []string{in, out})
	w.Spawn(a)
	ok, err := w.WaitExit(pid)
	if err != nil {
		return err.Error()
	}
	return ok
}

func (w *Worker) doReducer() error {
	isWork := true
	for isWork {
		sts, err := w.ReadDir("name/mr/r")
		if err != nil {
			log.Printf("Readdir reducer err %v\n", err)
			return err
		}
		isWork = false
		n := 0
		for _, st := range sts {
			log.Printf("try to claim %v\n", st.Name)
			_, err := w.PutFile("name/mr/r-claimed/"+st.Name, []byte{}, 0777|np.DMTMP, np.OWRITE)
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
