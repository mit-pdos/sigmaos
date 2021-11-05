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

type Reducer struct {
	*fslib.FsLib
	proc.ProcClnt
	reducef ReduceT
	crash   string
	input   string
	output  string
	name    string
}

func MakeReducer(reducef ReduceT, args []string) (*Reducer, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	r := &Reducer{}
	r.crash = args[0]
	r.input = args[1]
	r.output = args[2]
	r.reducef = reducef
	r.FsLib = fslib.MakeFsLib("reducer" + r.name)
	r.ProcClnt = procinit.MakeProcClnt(r.FsLib, procinit.GetProcLayersMap())
	log.Printf("MakeReducer %v\n", args)

	r.Started(procinit.GetPid())
	return r, nil
}

func (r *Reducer) processFile(file string) []KeyValue {
	kva := []KeyValue{}

	log.Printf("reduce %v\n", r.input+"/"+file)
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

	log.Printf("doReduce %v %v\n", r.input, r.output)

	_, err := r.ProcessDir(r.input, func(st *np.Stat) (bool, error) {
		kva = append(kva, r.processFile(st.Name)...)
		return false, nil
	})
	if err != nil {
		log.Fatalf("doReduce: ProcessDir %v err %v\n", r.input, err)
	}

	sort.Sort(ByKey(kva))

	if r.crash == "YES" {
		MaybeCrash()
	}

	// remove r.output file in case a crashed task left it behind
	r.Remove(r.output)
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
	r.Exited(procinit.GetPid(), "OK")
}
