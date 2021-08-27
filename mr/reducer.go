package mr

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

type ReduceT func(string, []string) string

const NReduce = 2

type Reducer struct {
	*fslib.FsLib
	proc.ProcCtl
	reducef ReduceT
	pid     string
	input   string
	output  string
	name    string
}

func MakeReducer(reducef ReduceT, args []string) (*Reducer, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	r := &Reducer{}
	db.Name("reducer")
	r.pid = args[0]
	r.input = args[1]
	r.output = args[2]
	r.reducef = reducef
	r.FsLib = fslib.MakeFsLib(r.name)
	r.ProcCtl = procinit.MakeProcCtl(r.FsLib, procinit.GetProcLayers())
	log.Printf("MakeReducer %v\n", args)
	r.Started(r.pid)
	return r, nil
}

func (r *Reducer) processFile(file string) []KeyValue {
	kva := []KeyValue{}

	db.DLPrintf("REDUCE", "reduce %v\n", r.input+"/"+file)
	fd, err := r.Open(r.input+"/"+file+"/", np.OREAD)
	if err != nil {
		log.Fatal("Open error ", err)
	}
	defer r.Close(fd)
	data, err := r.Read(fd, binary.MaxVarintLen64)
	if err != nil {
		log.Fatal(err)
	}
	rdr := bytes.NewReader(data)
	l, err := binary.ReadVarint(rdr)
	if err != nil {
		log.Fatal(err)
	}
	for l > 0 {
		data, err = r.Read(fd, np.Tsize(l))
		if err != nil {
			log.Fatal(err)
		}
		kvs := []KeyValue{}
		err = json.Unmarshal(data, &kvs)
		if err != nil {
			log.Fatal("Unmarshal error ", err)
		}
		db.DLPrintf("REDUCE", "reduce %v: kva %v\n", file, len(kvs))
		kva = append(kva, kvs...)

		data, err = r.Read(fd, binary.MaxVarintLen64)
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

func (r *Reducer) doReduce() error {
	kva := []KeyValue{}

	db.DLPrintf("REDUCE", "doReduce %v\n", r.input)
	r.ProcessDir(r.input, func(st *np.Stat) (bool, error) {
		kva = append(kva, r.processFile(st.Name)...)
		return false, nil
	})

	sort.Sort(ByKey(kva))

	fd, err := r.Create(r.output, 0777, np.OWRITE)
	if err != nil {
		return err
	}
	defer r.Close(fd)
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
		_, err = r.Write(fd, []byte(b))
		if err != nil {
			return err
		}
		i = j
	}
	return nil
}

func (r *Reducer) Work() {
	err := r.doReduce()
	if err != nil {
		log.Printf("doReduce error %v", err)
		os.Exit(1)
	}
}

func (r *Reducer) Exit() {
	r.Exited(r.pid)
}
