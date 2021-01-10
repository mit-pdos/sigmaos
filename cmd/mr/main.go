package main

import (
	"encoding/json"
	"hash/fnv"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"ulambda/fsclnt"
	// "ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

const (
	NReduce = 1
)

type Worker struct {
	clnt   *fsclnt.FsClient
	srv    *npsrv.NpServer
	memfsd *memfsd.Fsd
	done   chan bool
}

func makeWorker() *Worker {
	work := &Worker{}
	work.clnt = fsclnt.MakeFsClient("worker")
	work.memfsd = memfsd.MakeFsd()
	work.srv = npsrv.MakeNpServer(work.memfsd, ":0")
	work.done = make(chan bool)
	return work
}

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

func Map(filename string) []KeyValue {
	// XXX read file
	contents := ""
	// function to detect word separators.
	ff := func(r rune) bool { return !unicode.IsLetter(r) }

	// split contents into an array of words.
	words := strings.FieldsFunc(contents, ff)

	kva := []KeyValue{}
	for _, w := range words {
		kv := KeyValue{w, "1"}
		kva = append(kva, kv)
	}
	return kva
}

func (w *Worker) doMap(name string) {
	kvs := Map(name)
	base := filepath.Base(name)
	log.Print("doMap", name, " ", base)
	fds := []int{}
	offs := []np.Toffset{}
	for r := 0; r < NReduce; r++ {
		oname := "mr-" + base + "-" + strconv.Itoa(r)
		fd, err := w.clnt.Create("name/mr/reduce/"+oname, 0700, np.OWRITE)
		if err != nil {
			log.Fatal("doMap create error ", err)
		}
		fds = append(fds, fd)
		offs = append(offs, 0)
	}

	for _, kv := range kvs {
		r := ihash(kv.Key) % NReduce
		// XXX use append file?
		b, err := json.Marshal(kv)
		if err != nil {
			log.Fatal("doMap marshal error", err)
		}
		_, err = w.clnt.Write(fds[r], offs[r], b)
		if err != nil {
			log.Fatal("doMap write error ", err)
		}
		offs[r] += 1 // XXX len of kv
	}

	for _, fd := range fds {
		w.clnt.Close(fd)
	}
}

func pickOne(dirents []np.Stat) string {
	return dirents[0].Name
}

func (w *Worker) mPhase() {
	done := false
	for !done {
		fd, err := w.clnt.Opendir("name/mr/todo")
		if err != nil {
			log.Fatal("Opendir error ", err)
		}
		dirents, err := w.clnt.Readdir(fd, 0, 256)
		if err != nil && err != io.EOF {
			log.Fatal("Readdir error ", err)
		}
		log.Print("dirents ", dirents)
		w.clnt.Close(fd)
		if err == io.EOF { // are we done?
			fd, err := w.clnt.Opendir("name/mr/started")
			_, err = w.clnt.Readdir(fd, 0, 1024)
			if err != nil && err != io.EOF {
				log.Fatal("Readdir error ", err)
			}
			if err == io.EOF {
				done = true
			}
			log.Print("SPIN")
			w.clnt.Close(fd)
		} else {
			log.Print("pickone ", dirents)
			name := pickOne(dirents)
			err = w.clnt.Rename("name/mr/todo/"+name, "name/mr/started/"+name)
			if err == nil {
				w.doMap("name/mr/started/" + name)
				err := w.clnt.Remove("name/mr/started/" + name)
				if err != nil {
					log.Fatal("domap Remove error ", err)
				}
			}
		}
	}
	w.done <- true
}

func (w *Worker) doWork() {
	w.mPhase()
	// w.rPhase()
}

func main() {
	w := makeWorker()
	if fd, err := w.clnt.Attach(":1111", ""); err == nil {
		err := w.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
	}
	go w.doWork()
	<-w.done
	// work.clnt.Close(fd)
	log.Printf("Worker: finished\n")
}
