package mr

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	// "sort"
	"strconv"
	"strings"
	"time"

	//	"runtime"
	//	"runtime/debug"

	"github.com/dustin/go-humanize"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/sigmap"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/rand"
	"sigmaos/test"
	"sigmaos/writer"
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
	perf     *perf.Perf
}

func makeReducer(reducef ReduceT, args []string, p *perf.Perf) (*Reducer, error) {
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
	r.perf = p

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
	r.bwrt = bufio.NewWriterSize(w, np.BUFSZ)

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

func ReadKVs(rdr io.Reader, data Tdata) error {
	for {
		var kv KeyValue
		if err := DecodeKV(rdr, &kv); err != nil {
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
				break
			}
		}
		if _, ok := data[kv.Key]; !ok {
			data[kv.Key] = make([]string, 0)
		}
		data[kv.Key] = append(data[kv.Key], kv.Value)
	}
	return nil
}

// XXX cut new fslib?
func (r *Reducer) readFile(file string, data Tdata) (np.Tlength, time.Duration, bool) {
	// Make new fslib to parallelize request to a single fsux
	fsl := fslib.MakeFsLibAddr("r-"+file, fslib.Named())
	defer fsl.Exit()

	sym := r.input + "/" + file + "/"
	db.DPrintf("MR", "readFile %v\n", sym)
	rdr, err := fsl.OpenAsyncReader(sym, 0)
	if err != nil {
		db.DPrintf("MR", "MakeReader %v err %v", sym, err)
		return 0, 0, false
	}
	defer rdr.Close()

	start := time.Now()

	err = ReadKVs(rdr, data)
	db.DPrintf("MR0", "Reduce readfile %v %dms err %v\n", sym, time.Since(start).Milliseconds(), err)
	if err != nil {
		db.DPrintf("MR", "decodeKV %v err %v\n", sym, err)
		return 0, 0, false
	}
	return rdr.Nbytes(), time.Since(start), true
}

type Tdata map[string][]string

func (r *Reducer) readFiles(input string) (np.Tlength, time.Duration, Tdata, []string, error) {
	data := make(map[string][]string, 0)
	lostMaps := []string{}
	files := make(map[string]bool)
	nbytes := np.Tlength(0)
	duration := time.Duration(0)
	for len(files) < r.nmaptask {
		sts, err := r.ReadDirWatch(input, func(sts []*np.Stat) bool {
			return len(sts) == len(files)
		})
		if err != nil {
			return 0, 0, nil, nil, err
		}
		randOffset := int(rand.Uint64())
		if randOffset < 0 {
			randOffset *= -1
		}
		n := 0
		for i := range sts {
			// Random offset to stop reducers from all banging on the same ux.
			st := sts[(i+randOffset)%len(sts)]
			if _, ok := files[st.Name]; !ok {
				// Make sure we read an input file
				// only once.  Since mappers are
				// removing/creating files
				// concurrently from the directory we
				// also may have dup entries, so
				// filter here.
				files[st.Name] = true
				n += 1
				m, d, ok := r.readFile(st.Name, data)
				if !ok {
					lostMaps = append(lostMaps, strings.TrimPrefix(st.Name, "m-"))
				}
				//				runtime.GC()
				//				debug.FreeOSMemory()
				nbytes += m
				duration += d
			}
		}
	}
	return nbytes, duration, data, lostMaps, nil
}

func (r *Reducer) emit(kv *KeyValue) error {
	b := fmt.Sprintf("%s\t%s\n", kv.Key, kv.Value)
	_, err := r.bwrt.Write([]byte(b))
	return err
}

func (r *Reducer) doReduce() *proc.Status {
	db.DPrintf(db.ALWAYS, "doReduce %v %v %v\n", r.input, r.output, r.nmaptask)
	nin, duration, data, lostMaps, err := r.readFiles(r.input)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: readFiles %v err %v\n", proc.GetName(), r.input, err), nil)
	}
	if len(lostMaps) > 0 {
		return proc.MakeStatusErr(RESTART, lostMaps)
	}

	ms := duration.Milliseconds()
	fmt.Printf("reduce readfiles %s: in %s %vms (%s)\n", r.input, humanize.Bytes(uint64(nin)), ms, test.TputStr(nin, ms))

	start := time.Now()
	for k, vs := range data {
		if err := r.reducef(k, vs, r.emit); err != nil {
			return proc.MakeStatusErr("reducef", err)
		}
	}

	if err := r.bwrt.Flush(); err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: flush %v err %v\n", proc.GetName(), r.tmp, err), nil)
	}
	if err := r.wrt.Close(); err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: close %v err %v\n", proc.GetName(), r.tmp, err), nil)
	}
	// Include time spent writing output.
	duration += time.Since(start)
	err = r.Rename(r.tmp, r.output)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("%v: rename %v -> %v err %v\n", proc.GetName(), r.tmp, r.output, err), nil)
	}
	return proc.MakeStatusInfo(proc.StatusOK, r.input,
		Result{false, r.input, nin, r.wrt.Nbytes(), duration.Milliseconds()})
}

func RunReducer(reducef ReduceT, args []string) {
	p := perf.MakePerf("MRREDUCER")
	defer p.Done()

	r, err := makeReducer(reducef, args, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	status := r.doReduce()
	r.Exited(status)
}
