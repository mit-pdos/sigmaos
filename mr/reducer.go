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

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/delay"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
)

type ReduceT func(string, []string) string

type Reducer struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	reducef ReduceT
	crash   string
	input   string
	output  string
	tmp     string
}

func MakeReducer(reducef ReduceT, args []string) (*Reducer, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	r := &Reducer{}
	r.crash = args[0]
	r.input = args[1]
	r.output = args[2]
	r.tmp = r.output + rand.String(16)
	r.reducef = reducef
	r.FsLib = fslib.MakeFsLib("reducer-" + r.input)
	r.ProcClnt = procclnt.MakeProcClnt(r.FsLib)
	db.DPrintf("MakeReducer %v\n", args)

	r.Started(proc.GetPid())

	if r.crash == "YES" {
		crash.Crasher(r.FsLib, 500)
		delay.SetDelayRPC(3)
	}

	return r, nil
}

func (r *Reducer) processFile(file string) ([]KeyValue, error) {
	kva := []KeyValue{}

	d := r.input + "/" + file + "/"
	db.DPrintf("reduce %v\n", d)
	fd, err := r.Open(d, np.OREAD)
	if err != nil {
		// another reducer already completed; nothing to be done
		db.DPrintf("Open %v err %v", d, err)
		return nil, err
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
	return kva, nil
}

func (r *Reducer) doReduce() error {
	kva := []KeyValue{}

	db.DPrintf("doReduce %v %v\n", r.input, r.output)
	n := 0
	_, err := r.ProcessDir(r.input, func(st *np.Stat) (bool, error) {
		tkva, err := r.processFile(st.Name)
		if err != nil {
			return true, err
		}
		kva = append(kva, tkva...)
		n += 1
		return false, nil
	})
	if err != nil {
		db.DPrintf("doReduce: ProcessDir %v err %v\n", r.input, err)
		return nil
	}
	db.DPrintf("doReduce %v process %d files\n", r.input, n)

	sort.Sort(ByKey(kva))

	fd, err := r.Create(r.tmp, 0777, np.OWRITE)
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
	err = r.Rename(r.tmp, r.output)
	if err != nil {
		log.Fatalf("%v: rename %v -> %v failed %v\n", db.GetName(), r.tmp, r.output, err)
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
	r.Exited(proc.GetPid(), "OK")
}
