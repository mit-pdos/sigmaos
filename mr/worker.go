package mr

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"ulambda/fsclnt"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

const (
	NReduce = 1
)

type MapT func(string, string) []KeyValue
type ReduceT func(string, []string) string

type Worker struct {
	clnt    *fsclnt.FsClient
	memfsd  *memfsd.Fsd
	mapf    MapT
	reducef ReduceT
	Done    chan bool
}

func MakeWorker(mapf MapT, reducef ReduceT) *Worker {
	w := &Worker{}
	w.clnt = fsclnt.MakeFsClient("worker", false)
	w.memfsd = memfsd.MakeFsd(false)
	w.Done = make(chan bool)
	w.mapf = mapf
	w.reducef = reducef

	if fd, err := w.clnt.Attach(":1111", ""); err == nil {
		err := w.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
	}
	return w
}

func (w *Worker) readContents(name string) ([]byte, error) {
	const CNT = 8192
	fd, err := w.clnt.Open(name, np.OREAD)
	if err != nil {
		return nil, err
	}
	c := []byte{}
	for {
		b, err := w.clnt.Read(fd, CNT)
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			break
		}
		c = append(c, b...)
	}
	return c, nil
}

func (w *Worker) doMap(name string) {
	contents, err := w.readContents(name)
	if err != nil {
		log.Fatalf("readContents %v %v", name, err)
	}
	kvs := w.mapf(name, string(contents))
	base := filepath.Base(name)
	log.Print(os.Getpid(), " : ", " doMap", name)
	fds := []int{}
	for r := 0; r < NReduce; r++ {
		oname := "mr-" + base + "-" + strconv.Itoa(r)
		fd, err := w.clnt.Create("name/mr/reduce/"+oname, 0700, np.OWRITE)
		if err != nil {
			// maybe another worker finished earlier
			// XXX handle partial writing of intermediate files
			log.Printf("doMap create error %v %v\n", oname, err)
			return
		}
		fds = append(fds, fd)
	}

	for _, kv := range kvs {
		r := Khash(kv.Key) % NReduce
		// XXX use append file?
		b, err := json.Marshal(kv)
		if err != nil {
			log.Fatal("doMap marshal error", err)
		}
		_, err = w.clnt.Write(fds[r], b)
		if err != nil {
			log.Fatal("doMap write error ", err)
		}
	}

	for _, fd := range fds {
		w.clnt.Close(fd)
	}
}

func (w *Worker) isEmpty(name string) bool {
	st, err := w.clnt.Stat(name)
	if err != nil {
		log.Fatalf("Stat %v error %v\n", name, err)
	}
	return st.Length == 0
}

func (w *Worker) doTask(name string) {
	err := w.clnt.Rename("name/mr/todo/"+name, "name/mr/started/"+name)
	if err == nil {
		w.doMap("name/mr/started/" + name)
		err := w.clnt.Remove("name/mr/started/" + name)
		if err != nil {
			// controler may have reclaimed the task
			// or another worked may have finished it earlier
			log.Printf("domap Remove %v error %v\n", name, err)
		}
	}
}

func (w *Worker) mPhase() {
	done := false
	for !done {
		fd, err := w.clnt.Opendir("name/mr/todo")
		if err != nil {
			log.Fatal("Opendir error ", err)
		}
		dirents, err := w.clnt.Readdir(fd, 256)
		if err != nil && err != io.EOF {
			log.Fatal("Readdir error ", err)
		}
		w.clnt.Close(fd)
		if err == io.EOF { // are we done?
			if w.isEmpty("name/mr/started") {
				done = true
			} else {
				// log.Print("SPIN")
			}
		} else {
			w.doTask(dirents[0].Name)
		}
	}
}

func (w *Worker) Work() {
	w.mPhase()
	w.Done <- true
}
