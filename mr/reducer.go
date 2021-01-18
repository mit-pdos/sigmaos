package mr

import (
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
	m := &Reducer{}
	m.clnt = fslib.MakeFsLib(false)
	m.reducef = reducef
	if len(args) != 3 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	log.Printf("MakeReducer %v\n", args)
	m.pid = args[0]
	m.input = args[1]
	m.output = args[2]
	return m, nil
}

func (r *Reducer) doReduce() {
	kva := []KeyValue{}

	log.Printf("doReduce %v\n", r.input)
	r.clnt.ProcessDir(r.input, func(st *np.Stat) bool {
		data, err := r.clnt.ReadFile(r.input + "/" + st.Name)
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
		return false
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
}
