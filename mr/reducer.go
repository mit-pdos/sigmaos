package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	input   string
	output  string
}

func MakeReducer(reducef ReduceT, inputs []string) (*Reducer, error) {
	m := &Reducer{}
	m.clnt = fslib.MakeFsLib(false)
	m.reducef = reducef
	if len(inputs) != 2 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	log.Printf("MakeReducer %v\n", inputs)
	m.input = inputs[0]
	m.output = inputs[1]
	return m, nil
}

// XXX use directory reading library function
func (r *Reducer) doReduce() {
	kva := []KeyValue{}

	log.Printf("doReduce %v\n", r.input)
	fd, err := r.clnt.Opendir(r.input)
	if err != nil {
		log.Fatal("Opendir error ", err)
	}
	for {
		dirents, err := r.clnt.Readdir(fd, 256)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("Readdir error ", err)
		}
		for _, st := range dirents {
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
		}
	}
	r.clnt.Close(fd)

	sort.Sort(ByKey(kva))

	fd, err = r.clnt.Create(r.output, 0777, np.OWRITE)
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

		// output is an array of strings.
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
