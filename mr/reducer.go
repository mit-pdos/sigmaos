package mr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	//	"runtime"
	//	"runtime/debug"

	"github.com/dustin/go-humanize"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/writer"
)

type Reducer struct {
	*sigmaclnt.SigmaClnt
	reducef      ReduceT
	input        string
	outputTarget string
	outlink      string
	nmaptask     int
	tmp          string
	pwrt         *perf.PerfWriter
	asyncwrt     *fslib.Wrt
	syncwrt      *writer.Writer
	perf         *perf.Perf
	asyncrw      bool
}

func NewReducer(sc *sigmaclnt.SigmaClnt, reducef ReduceT, args []string, p *perf.Perf) (*Reducer, error) {
	r := &Reducer{
		input:        args[0],
		outlink:      args[1],
		outputTarget: args[2],
		reducef:      reducef,
		SigmaClnt:    sc,
		perf:         p,
	}
	asyncrw, err := strconv.ParseBool(args[4])
	if err != nil {
		return nil, fmt.Errorf("NewReducer: can't parse asyncrw %v", args[3])
	}
	r.asyncrw = asyncrw
	r.tmp = r.outputTarget + rand.String(16) //pn

	db.DPrintf(db.MR, "Reducer outputting to %v", r.tmp)

	m, err := strconv.Atoi(args[3])
	if err != nil {
		return nil, fmt.Errorf("Reducer: nmaptask %v isn't int", args[2])
	}
	r.nmaptask = m

	sc.MkDir(filepath.Dir(r.tmp), 0777)
	if r.asyncrw {
		w, err := r.CreateAsyncWriter(r.tmp, 0777, sp.OWRITE)
		if err != nil {
			db.DFatalf("Error CreateWriter [%v] %v", r.tmp, err)
			return nil, err
		}
		r.asyncwrt = w
		r.pwrt = perf.NewPerfWriter(r.asyncwrt, r.perf)
	} else {
		w, err := r.CreateWriter(r.tmp, 0777, sp.OWRITE)
		if err != nil {
			db.DFatalf("Error CreateWriter [%v] %v", r.tmp, err)
			return nil, err
		}
		r.syncwrt = w
		r.pwrt = perf.NewPerfWriter(r.syncwrt, r.perf)
	}
	return r, nil
}

func newReducer(reducef ReduceT, args []string, p *perf.Perf) (*Reducer, error) {
	if len(args) != 5 {
		return nil, errors.New("NewReducer: too few arguments")
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, fmt.Errorf("NewReducer: can't parse asyncrw %v", args[3])
	}
	r, err := NewReducer(sc, reducef, args, p)
	if err != nil {
		return nil, err
	}
	if err := r.Started(); err != nil {
		return nil, fmt.Errorf("NewReducer couldn't start %v err %v", args, err)
	}
	crash.Crasher(r.FsLib)
	return r, nil
}

type result struct {
	kvs  []*KeyValue
	name string
	ok   bool
	n    sp.Tlength
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

func (r *Reducer) readFile(file string, data Tdata) (sp.Tlength, time.Duration, bool) {
	// Make new fslib to parallelize request to a single fsux
	pe := proc.NewAddedProcEnv(r.ProcEnv())
	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, r.GetNetProxyClnt())
	if err != nil {
		db.DPrintf(db.MR, "NewSigmaClntFsLib err %v", err)
		return 0, 0, false
	}
	defer sc.Close()

	sym := r.input + "/" + file + "/"
	db.DPrintf(db.MR, "readFile %v\n", sym)
	rdr, err := sc.OpenAsyncReader(sym, 0)
	if err != nil {
		db.DPrintf(db.MR, "NewReader %v err %v", sym, err)
		return 0, 0, false
	}
	defer rdr.Close()

	start := time.Now()

	err = ReadKVs(rdr, data)
	db.DPrintf(db.MR, "Reduce readfile %v %dms err %v\n", sym, time.Since(start).Milliseconds(), err)
	if err != nil {
		db.DPrintf(db.MR, "decodeKV %v err %v\n", sym, err)
		return 0, 0, false
	}
	return rdr.Nbytes(), time.Since(start), true
}

type Tdata map[string][]string

func (r *Reducer) ReadFiles() (sp.Tlength, time.Duration, Tdata, []string, error) {
	data := make(map[string][]string, 0)
	lostMaps := []string{}
	nfile := 0
	nbytes := sp.Tlength(0)
	duration := time.Duration(0)
	str, err := r.SprintfDir("name/ux/~local/")
	if err != nil {
		db.DPrintf(db.MR, "SprintfDir %v err %v", r.input, err)
		return 0, 0, nil, nil, err
	}
	db.DPrintf(db.MR, "Readfiles %v", str)
	dr := fslib.NewDirReader(r.FsLib, r.input)
	for nfile < r.nmaptask {
		files, err := dr.WatchNewUniqueEntries()
		if err != nil {
			db.DPrintf(db.MR, "Watch err %v", err)
			return 0, 0, nil, nil, err
		}
		db.DPrintf(db.MR, "files %v", files)
		randOffset := int(rand.Uint64())
		if randOffset < 0 {
			randOffset *= -1
		}
		for i, _ := range files {
			// Random offset to stop reducers from all banging on the same ux.
			f := files[(i+randOffset)%len(files)]
			m, d, ok := r.readFile(f, data)
			if !ok {
				lostMaps = append(lostMaps, strings.TrimPrefix(f, "m-"))
			}
			//				runtime.GC()
			//				debug.FreeOSMemory()
			nbytes += m
			duration += d
			nfile += 1
		}
	}
	return nbytes, duration, data, lostMaps, nil
}

func (r *Reducer) emit(key []byte, value string) error {
	b := fmt.Sprintf("%s\t%s\n", key, value)
	_, err := r.pwrt.Write([]byte(b))
	if err != nil {
		db.DPrintf(db.ALWAYS, "Err emt write bwriter: %v", err)
	}
	return err
}

func (r *Reducer) DoReduce() *proc.Status {
	db.DPrintf(db.ALWAYS, "DoReduce %v %v %v\n", r.input, r.outlink, r.nmaptask)
	nin, duration, data, lostMaps, err := r.ReadFiles()
	if err != nil {
		db.DPrintf(db.ALWAYS, "ReadFiles: err %v", err)
		return proc.NewStatusErr(fmt.Sprintf("%v: ReadFiles %v err %v\n", r.ProcEnv().GetPID(), r.input, err), nil)
	}
	if len(lostMaps) > 0 {
		return proc.NewStatusErr(RESTART, lostMaps)
	}

	ms := duration.Milliseconds()
	db.DPrintf(db.ALWAYS, "reduce readfiles %s: in %s %vms (%s)\n", r.input, humanize.Bytes(uint64(nin)), ms, test.TputStr(nin, ms))

	start := time.Now()
	for k, vs := range data {
		if err := r.reducef(k, vs, r.emit); err != nil {
			db.DPrintf(db.ALWAYS, "Err reducef: %v", err)
			return proc.NewStatusErr("reducef", err)
		}
	}

	var nbyte sp.Tlength
	if r.asyncrw {
		if err := r.asyncwrt.Close(); err != nil {
			return proc.NewStatusErr(fmt.Sprintf("%v: close %v err %v\n", r.ProcEnv().GetPID(), r.tmp, err), nil)
		}
		nbyte = r.asyncwrt.Nbytes()
	} else {
		if err := r.syncwrt.Close(); err != nil {
			return proc.NewStatusErr(fmt.Sprintf("%v: close %v err %v\n", r.ProcEnv().GetPID(), r.tmp, err), nil)
		}
		nbyte = r.syncwrt.Nbytes()
	}

	// Include time spent writing output.
	duration += time.Since(start)

	// Create symlink atomically.
	if err := r.PutFileAtomic(r.outlink, 0777|sp.DMSYMLINK, []byte(r.tmp)); err != nil {
		return proc.NewStatusErr(fmt.Sprintf("%v: put symlink %v -> %v err %v\n", r.ProcEnv().GetPID(), r.outlink, r.tmp, err), nil)
	}
	return proc.NewStatusInfo(proc.StatusOK, r.input,
		Result{false, r.input, nin, nbyte, duration.Milliseconds()})
}

func RunReducer(reducef ReduceT, args []string) {
	pe := proc.GetProcEnv()
	p, err := perf.NewPerf(pe, perf.MRREDUCER)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	db.DPrintf(db.BENCH, "Reducer time since spawn %v", time.Since(pe.GetSpawnTime()))
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v\n", err)
	}
	r, err := NewReducer(sc, reducef, args, p)
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}

	status := r.DoReduce()
	r.ClntExit(status)
}
