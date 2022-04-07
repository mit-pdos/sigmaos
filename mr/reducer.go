package mr

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/delay"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
	"ulambda/writer"
)

type Reducer struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	reducef ReduceT
	input   string
	output  string
	tmp     string
	bwrt    *bufio.Writer
	wrt     *writer.Writer
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

	w, err := r.CreateWriter(r.tmp, 0777, np.OWRITE)
	if err != nil {
		return nil, err
	}
	r.wrt = w
	r.bwrt = bufio.NewWriterSize(w, BUFSZ)

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

	start := time.Now()
	err = fslib.JsonBufReader(bufio.NewReaderSize(rdr, BUFSZ), func() interface{} { return new(KeyValue) }, func(a interface{}) error {
		kv := a.(*KeyValue)
		db.DPrintf("REDUCE", "reduce %v: kv %v\n", file, kv)
		kva = append(kva, kv)
		return nil
	})
	if err != nil {
		return nil, err
	}
	db.DPrintf("MR0", "Reduce Read %v\n", time.Since(start).Milliseconds())
	return kva, nil
}

func (r *Reducer) processDirs(input string) ([]*KeyValue, error) {
	return nil, nil
}

func (r *Reducer) emit(kv *KeyValue) error {
	b := fmt.Sprintf("%v %v\n", kv.Key, kv.Value)
	_, err := r.bwrt.Write([]byte(b))
	return err
}

func (r *Reducer) doReduce() *proc.Status {
	kva := []*KeyValue{}
	lostMaps := []string{}

	db.DPrintf(db.ALWAYS, "doReduce %v %v\n", r.input, r.output)
	n := 0
	_, err := r.ProcessDir(r.input, func(st *np.Stat) (bool, error) {
		log.Printf("name %v\n", st.Name)
		tkva, err := r.processFile(st.Name)
		if err != nil {
			// If error is true, then either another
			// reducer already did the job (the input dir
			// is missing), the server holding the
			// mapper's output crashed, or is unreachable (in
			// which case we need to restart that mapper).
			lostMaps = append(lostMaps, strings.TrimPrefix(st.Name, "m-"))
		}
		kva = append(kva, tkva...)
		n += 1
		return false, nil
	})
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: ProcessDir %v err %v\n", proc.GetName(), r.input, err), nil)
	}

	if len(lostMaps) > 0 {
		log.Printf("lost maps %v\n", lostMaps)
		return proc.MakeStatusErr(RESTART, lostMaps)
	}

	start := time.Now()
	sort.Sort(ByKey(kva))
	db.DPrintf("MR0", "Reduce Sort %v\n", time.Since(start).Milliseconds())

	start = time.Now()
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
		if err := r.reducef(kva[i].Key, values, r.emit); err != nil {
			return proc.MakeStatusErr("reducef", err)
		}
		i = j
	}

	if err := r.bwrt.Flush(); err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: flush %v err %v\n", proc.GetName(), r.tmp, err), nil)
	}
	if err := r.wrt.Close(); err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: close %v err %v\n", proc.GetName(), r.tmp, err), nil)
	}
	err = r.Rename(r.tmp, r.output)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: rename %v -> %v err %v\n", proc.GetName(), r.tmp, r.output, err), nil)
	}
	db.DPrintf("MR0", "Reduce reduce %v\n", time.Since(start).Milliseconds())

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
