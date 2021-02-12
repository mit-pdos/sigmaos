package mr

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"

	"ulambda/fslib"
	np "ulambda/ninep"
)

type ReduceT func(string, []string) string

const NReduce = 1

type Reducer struct {
	clnt    *fslib.FsLib
	reducef ReduceT
	pid     string
	input   string
	output  string
}

func MakeReducer(reducef ReduceT, args []string) (*Reducer, error) {
	r := &Reducer{}
	r.clnt = fslib.MakeFsLib("reducer")
	r.reducef = reducef
	if len(args) != 3 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	log.Printf("MakeReducer %v\n", args)
	r.pid = args[0]
	r.input = args[1]
	r.output = args[2]
	r.clnt.Started(r.pid)
	return r, nil
}

func (r *Reducer) processFile(file string) []KeyValue {
	kva := []KeyValue{}

	fd, err := r.clnt.Open(r.input+"/"+file, np.OREAD)
	if err != nil {
		log.Fatal("Open error ", err)
	}
	defer r.clnt.Close(fd)
	data, err := r.clnt.Read(fd, binary.MaxVarintLen64)
	if err != nil {
		log.Fatal(err)
	}
	rdr := bytes.NewReader(data)
	l, err := binary.ReadVarint(rdr)
	if err != nil {
		log.Fatal(err)
	}
	for l > 0 {
		data, err = r.clnt.Read(fd, np.Tsize(l))
		if err != nil {
			log.Fatal(err)
		}
		kvs := []KeyValue{}
		err = json.Unmarshal(data, &kvs)
		if err != nil {
			log.Fatal("Unmarshal error ", err)
		}
		// log.Printf("reduce %v: kva %v\n", file, len(kvs))
		kva = append(kva, kvs...)

		data, err = r.clnt.Read(fd, binary.MaxVarintLen64)
		if err != nil {
			log.Fatal(err)
		}
		if len(data) == 0 {
			break
		}
		rdr = bytes.NewReader(data)
		l, err = binary.ReadVarint(rdr)
		if err != nil {
			log.Fatal(err)
		}
	}
	return kva
}

func (r *Reducer) doReduce() {
	kva := []KeyValue{}

	log.Printf("doReduce %v\n", r.input)
	r.clnt.ProcessDir(r.input, func(st *np.Stat) (bool, error) {
		kva = append(kva, r.processFile(st.Name)...)
		return false, nil
	})

	sort.Sort(ByKey(kva))

	fd, err := r.clnt.Create(r.output, 0777, np.OWRITE)
	if err != nil {
		log.Fatal("Create error ", err)
	}
	defer r.clnt.Close(fd)
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
		output := r.reducef(kva[i].Key, values)
		b := fmt.Sprintf("%v %v\n", kva[i].Key, output)
		_, err = r.clnt.Write(fd, []byte(b))
		if err != nil {
			log.Fatal("Write error ", err)
		}
		i = j
	}
}

func (r *Reducer) Work() {
	r.doReduce()
	r.clnt.Exiting(r.pid, "OK")
}
