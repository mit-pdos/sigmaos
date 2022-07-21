package mr

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	// "github.com/klauspost/readahead"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
	"ulambda/test"
	"ulambda/writer"
)

type Reducer struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	reducef  ReduceT
	input    string
	output   string
	nmaptask int
	tmp      string
	bwrt     *bufio.Writer
	wrt      *writer.Writer
}

func makeReducer(reducef ReduceT, args []string) (*Reducer, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReducer: too few arguments")
	}
	r := &Reducer{}
	r.input = args[0]
	r.output = args[1]
	r.tmp = r.output + rand.String(16)
	r.reducef = reducef
	r.FsLib = fslib.MakeFsLib("reducer-" + r.input)
	r.ProcClnt = procclnt.MakeProcClnt(r.FsLib)

	m, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("Reducer: nmaptask %v isn't int", args[2])
	}
	r.nmaptask = m

	w, err := r.CreateWriter(r.tmp, 0777, np.OWRITE)
	if err != nil {
		return nil, err
	}
	r.wrt = w
	r.bwrt = bufio.NewWriterSize(w, test.BUFSZ)

	if err := r.Started(); err != nil {
		return nil, fmt.Errorf("MakeReducer couldn't start %v", args)
	}

	crash.Crasher(r.FsLib)
	return r, nil
}

type result struct {
	kvs  []*KeyValue
	name string
	ok   bool
	n    np.Tlength
}

func (r *Reducer) readFile(ch chan result, file string) {
	// Make new fslib to parallelize request to a single fsux
	fsl := fslib.MakeFsLibAddr("r-"+file, fslib.Named())
	defer fsl.Exit()

	kvs := make([]*KeyValue, 0)
	sym := r.input + "/" + file + "/"
	db.DPrintf("MR", "readFile %v\n", sym)
	rdr, err := fsl.OpenReader(sym)
	if err != nil {
		db.DPrintf("MR", "MakeReader %v err %v", sym, err)
		ch <- result{nil, file, false, 0}
		return
	}
	defer rdr.Close()
	start := time.Now()

	brdr := bufio.NewReaderSize(rdr, test.BUFSZ)
	//ardr, err := readahead.NewReaderSize(rdr, 4, test.BUFSZ)
	//if err != nil {
	//	db.DFatalf("%v: readahead.NewReaderSize err %v", proc.GetName(), err)
	//}
	err = fslib.JsonReader(brdr, func() interface{} { return new(KeyValue) }, func(a interface{}) error {
		kv := a.(*KeyValue)
		db.DPrintf("REDUCE1", "reduce %v/%v: kv %v\n", r.input, file, kv)
		kvs = append(kvs, kv)
		return nil
	})
	if err != nil {
		db.DPrintf("MR", "JsonReader %v err %v\n", sym, err)
		ch <- result{nil, file, false, 0}
	} else {
		ch <- result{kvs, file, true, rdr.Nbytes()}
	}
	db.DPrintf("MR0", "Reduce readfile %v %v\n", sym, time.Since(start).Milliseconds())
	return
}

func (r *Reducer) readFiles(input string) (np.Tlength, []*KeyValue, []string, error) {
	kvs := []*KeyValue{}
	lostMaps := []string{}
	files := make(map[string]bool)
	nbytes := np.Tlength(0)
	for len(files) < r.nmaptask {
		sts, err := r.ReadDirWatch(input, func(sts []*np.Stat) bool {
			return len(sts) == len(files)
		})
		if err != nil {
			return 0, nil, nil, err
		}
		n := 0
		ch := make(chan result)
		for _, st := range sts {
			if _, ok := files[st.Name]; !ok {
				// Make sure we read an input file
				// only once.  Since mappers are
				// removing/creating files
				// concurrently from the directory we
				// also may have dup entries, so
				// filter here.
				files[st.Name] = true
				n += 1
				go r.readFile(ch, st.Name)
			}
		}
		for i := 0; i < n; i++ {
			res := <-ch
			db.DPrintf("REDUCE", "Read %v %v ok %v\n", r.input, res.name, res.ok)
			if !res.ok {
				// If !ok, then readFile failed to
				// read input shard, perhaps the
				// server holding the mapper's output
				// crashed, or is unreachable. Keep
				// track that we need to restart that
				// mappers, but keep going processing
				// other shards to see if more mappers
				// need to be restarted.
				lostMaps = append(lostMaps, strings.TrimPrefix(res.name, "m-"))
			} else {
				nbytes += res.n
				kvs = append(kvs, res.kvs...)
			}
		}
	}
	return nbytes, kvs, lostMaps, nil
}

func (r *Reducer) emit(kv *KeyValue) error {
	b := fmt.Sprintf("%v %v\n", kv.K, kv.V)
	_, err := r.bwrt.Write([]byte(b))
	return err
}

func (r *Reducer) doReduce() *proc.Status {
	db.DPrintf(db.ALWAYS, "doReduce %v %v %v\n", r.input, r.output, r.nmaptask)
	start := time.Now()
	nin, kvs, lostMaps, err := r.readFiles(r.input)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: readFiles %v err %v\n", proc.GetName(), r.input, err), nil)
	}
	if len(lostMaps) > 0 {
		return proc.MakeStatusErr(RESTART, lostMaps)
	}

	sstart := time.Now()
	sort.Sort(ByKey(kvs))
	db.DPrintf("MR0", "Reduce Sort %v\n", time.Since(sstart).Milliseconds())

	i := 0
	for i < len(kvs) {
		j := i + 1
		for j < len(kvs) && kvs[j].K == kvs[i].K {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, kvs[k].V)
		}
		if err := r.reducef(kvs[i].K, values, r.emit); err != nil {
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
	return proc.MakeStatusInfo(proc.StatusOK, r.input,
		Result{false, r.input, nin, r.wrt.Nbytes(), time.Since(start).Milliseconds()})
}

func RunReducer(reducef ReduceT, args []string) {
	p := perf.MakePerf("MR-REDUCER")
	defer p.Done()

	r, err := makeReducer(reducef, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	status := r.doReduce()
	r.Exited(status)
}
