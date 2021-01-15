package mr

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"ulambda/fsclnt"
	np "ulambda/ninep"
)

const (
	NReduce    = 1
	INPROGRESS = "name/mr/started"
)

type ReduceT func(string, []string) string

type Worker struct {
	clnt    *fsclnt.FsClient
	mapf    MapT
	reducef ReduceT
}

func MakeWorker(mapf MapT, reducef ReduceT) *Worker {
	w := &Worker{}
	w.clnt = fsclnt.MakeFsClient(false)
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

// XXX fslib with support routines?
func (w *Worker) readFile(fname string) ([]byte, error) {
	const CNT = 8192
	fd, err := w.clnt.Open(fname, np.OREAD)
	if err != nil {
		log.Printf("open failed %v %v\n", fname, err)
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
	err = w.clnt.Close(fd)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (w *Worker) makeFile(fname string, data []byte) error {
	log.Printf("makeFile %v\n", fname)
	fd, err := w.clnt.Create(fname, 0700, np.OWRITE)
	if err != nil {
		return err
	}
	_, err = w.clnt.Write(fd, data)
	if err != nil {
		return err
	}
	return w.clnt.Close(fd)
}

func (w *Worker) readData(task string) ([]byte, error) {
	const CNT = 8192
	log.Printf("readData task %v\n", task)
	file, err := w.readFile(task)
	if err != nil {
		return nil, err
	}
	return w.readFile(strings.TrimRight(string(file), "\n\r"))
}

func (w *Worker) doMap(name string) {
	contents, err := w.readData(name)
	if err != nil {
		log.Fatalf("readContents %v %v", name, err)
	}
	kvs := w.mapf(name, string(contents))
	log.Printf("%v: kvs = %v\n", name, len(kvs))
	base := filepath.Base(name)
	log.Print(os.Getpid(), " : ", " doMap ", name)

	// split
	skvs := make([][]KeyValue, NReduce)
	for _, kv := range kvs {
		r := Khash(kv.Key) % NReduce
		skvs[r] = append(skvs[r], kv)
	}

	for r := 0; r < NReduce; r++ {
		b, err := json.Marshal(skvs[r])
		if err != nil {
			log.Fatal("doMap marshal error", err)
		}
		oname := "name/mr/reduce/" + strconv.Itoa(r) + "/mr-" + base
		err = w.makeFile(oname, b)
		if err != nil {
			// maybe another worker finished earlier
			// XXX handle partial writing of intermediate files
			log.Printf("doMap create error %v %v\n", oname, err)
			return
		}
		log.Printf("new reduce task %v %v\n", oname, len(skvs[r]))
	}

}

// XXX factor directory reading in library function
func (w *Worker) doReduce(name string) {
	kva := []KeyValue{}

	log.Printf("doReduce %v\n", name)
	fd, err := w.clnt.Opendir(name)
	if err != nil {
		log.Fatal("Opendir error ", err)
	}
	for {
		dirents, err := w.clnt.Readdir(fd, 256)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("Readdir error ", err)
		}
		for _, st := range dirents {
			data, err := w.readFile(name + "/" + st.Name)
			if err != nil {
				log.Fatal("readFile error ", err)
			}
			kvs := []KeyValue{}
			err = json.Unmarshal(data, &kvs)
			if err != nil {
				log.Fatal("Unmarshal error ", err)
			}
			log.Printf("reduce %v: kva %v\n", st.Name, len(kvs))
			kva = append(kva, kvs...)
		}
	}
	w.clnt.Close(fd)

	sort.Sort(ByKey(kva))

	oname := "mr-out-" + strconv.Itoa(0)
	fd, err = w.clnt.Create("name/mr/"+oname, 0777, np.OWRITE)
	if err != nil {
		log.Fatal("Create error ", err)
	}
	defer w.clnt.Close(fd)
	i := 0
	for i < len(kva) {
		j := i + 1
		for j < len(kva) && kva[j].Key == kva[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, kva[k].Value)
		}
		output := w.reducef(kva[i].Key, values)

		// output is an array of strings.
		b := fmt.Sprintf("%v %v\n", kva[i].Key, output)

		_, err = w.clnt.Write(fd, []byte(b))
		if err != nil {
			log.Fatal("Write error ", err)
		}
		i = j
	}
}

func (w *Worker) isEmpty(name string) bool {
	st, err := w.clnt.Stat(name)
	if err != nil {
		log.Fatalf("Stat %v error %v\n", name, err)
	}
	return st.Length == 0
}

func (w *Worker) doTask(path, name string) {
	err := w.clnt.Rename(path+"/"+name, INPROGRESS+"/"+name)
	if err != nil {
		// another worker may have grabbed this task
		log.Print("Rename failed ", path+"/"+name, " ", err)
	} else {
		log.Printf("renamed to %v\n", INPROGRESS+"/"+name)
		if path == "name/mr/map" {
			w.doMap(INPROGRESS + "/" + name)
		} else {
			w.doReduce(INPROGRESS + "/" + name)
		}
		err := w.clnt.Remove(INPROGRESS + "/" + name)
		if err != nil {
			// monitor may have reclaimed the task or
			// another worked may have finished it earlier
			log.Printf("domap Remove %v error %v\n", name, err)
		}
	}
}

func (w *Worker) Phase(path string) {
	done := false
	for !done {
		fd, err := w.clnt.Opendir(path)
		if err != nil {
			log.Fatal("Opendir error ", err)
		}
		dirents, err := w.clnt.Readdir(fd, 256)
		if err == io.EOF {
			if w.isEmpty(INPROGRESS) {
				done = true
			} else {
				fmt.Printf(".")
			}
		} else {
			if err != nil {
				log.Fatal("Readdir error ", err)
			}
			w.doTask(path, dirents[0].Name)

		}
		w.clnt.Close(fd)
	}
}

func (w *Worker) Work() {
	w.Phase("name/mr/map")
	w.Phase("name/mr/reduce")
}
