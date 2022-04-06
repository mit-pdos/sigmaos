package mr

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

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
	input   string
	output  string
	tmp     string
}

func makeReducer(reducef ReduceT, args []string) (*Reducer, error) {
	if len(args) != 2 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	r := &Reducer{}
	r.input = args[0]
	r.output = args[1]
	r.tmp = r.output + rand.String(16)
	r.reducef = reducef
	r.FsLib = fslib.MakeFsLib("reducer-" + r.input)
	r.ProcClnt = procclnt.MakeProcClnt(r.FsLib)

	r.Started()

	crash.Crasher(r.FsLib)
	delay.SetDelayRPC(3)

	return r, nil
}

func (r *Reducer) processFile(file string) ([]*KeyValue, error) {
	kva := make([]*KeyValue, 0)

	d := r.input + "/" + file + "/"
	db.DPrintf("MR", "reduce %v\n", d)
	rdr, err := r.OpenReader(d)
	if err != nil {
		// another reducer already completed; nothing to be done
		db.DPrintf("MR", "MakeReader %v err %v", d, err)
		return nil, err
	}
	defer rdr.Close()
	err = rdr.ReadJsonStream(func() interface{} { return new(KeyValue) }, func(a interface{}) error {
		kv := a.(*KeyValue)
		db.DPrintf("REDUCE", "reduce %v: kva %v\n", file, kv)
		kva = append(kva, kv)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return kva, nil
}

func (r *Reducer) doReduce() *proc.Status {
	kva := []*KeyValue{}
	lostMaps := []string{}

	log.Printf("%v: doReduce %v %v\n", proc.GetName(), r.input, r.output)
	n := 0
	_, err := r.ProcessDir(r.input, func(st *np.Stat) (bool, error) {
		tkva, err := r.processFile(st.Name)
		if err != nil {
			// If error is true, then either another
			// reducer already did the job (the input dir
			// is missing) or the server holding the
			// mapper's output crashed or is unreachable
			// (in which case we need to restart that
			// mapper).
			lostMaps = append(lostMaps, strings.TrimPrefix(st.Name, "m-"))
		}
		log.Printf("processFile %v\n", err)
		kva = append(kva, tkva...)
		n += 1
		return false, nil
	})
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: ProcessDir %v err %v\n", proc.GetName(), r.input, err), nil)
	}

	if len(lostMaps) > 0 {
		return proc.MakeStatusErr(RESTART, lostMaps)
	}

	sort.Sort(ByKey(kva))

	fd, err := r.Create(r.tmp, 0777, np.OWRITE)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: create %v err %v\n", proc.GetName(), r.tmp, err), nil)
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
			return proc.MakeStatusErr(fmt.Sprintf("%v: write %v err %v\n", proc.GetName(), r.tmp, err), nil)
		}
		i = j
	}
	err = r.Rename(r.tmp, r.output)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: rename %v -> %v err %v\n", proc.GetName(), r.tmp, r.output, err), nil)
	}
	return proc.MakeStatus(proc.StatusOK)
}

func RunReducer(reducef ReduceT, args []string) {
	r, err := makeReducer(reducef, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	status := r.doReduce()
	r.Exited(status)
}
