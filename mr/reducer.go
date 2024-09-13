package mr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

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
	r.tmp = r.outputTarget + rand.String(16)

	db.DPrintf(db.MR, "Reducer outputting to %v %t", r.tmp, r.asyncrw)

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

func ReadKVs(rdr io.Reader, kvm *kvmap, reducef ReduceT) error {
	kvd := newKVDecoder(rdr, 1000, 10000)
	for {
		if k, v, err := kvd.decode(); err != nil {
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
				break
			}
		} else {
			if err := kvm.combine(k, v, reducef); err != nil {
				return err
			}
		}
	}
	return nil
}

type readResult struct {
	f          string
	ok         bool
	n          sp.Tlength
	d          time.Duration
	kvm        *kvmap
	mapsFailed []string
}

func (rtot *readResult) sum(r *readResult) {
	db.DPrintf(db.MR, "sum %q %t %v %v", r.f, r.ok, r.n, r.d)
	if !r.ok {
		rtot.mapsFailed = append(rtot.mapsFailed, strings.TrimPrefix(r.f, "m-"))
	} else {
		rtot.n += r.n
		rtot.d += r.d
	}
}

func (r *Reducer) readFile(rr *readResult) {
	sym := r.input + "/" + rr.f + "/"
	db.DPrintf(db.MR, "readFile %v\n", sym)
	rdr, err := r.OpenAsyncReader(sym, 0)
	if err != nil {
		db.DPrintf(db.MR, "NewReader %v err %v", sym, err)
		rr.ok = false
		return
	}
	defer rdr.Close()

	start := time.Now()
	err = ReadKVs(rdr, rr.kvm, r.reducef)
	db.DPrintf(db.MR, "Reduce readfile %v %dms err %v\n", sym, time.Since(start).Milliseconds(), err)
	if err != nil {
		db.DPrintf(db.MR, "decodeKV %v err %v\n", sym, err)
		rr.ok = false
		return
	}
	rr.n = rdr.Nbytes()
	rr.d = time.Since(start)
	rr.ok = true
}

func (r *Reducer) readerMgr(req chan string, rep chan readResult, max int) {
	mu := &sync.Mutex{}
	producer := sync.NewCond(mu)
	n := 0

	for f := range req {
		mu.Lock()
		n += 1
		for n > max {
			producer.Wait()
		}
		mu.Unlock()
		db.DPrintf(db.MR, "readerMgr: start %q", f)
		go func(f string) {
			kvm := newKvmap(MINCAP, MAXCAP)
			rr := readResult{f: f, kvm: kvm}
			r.readFile(&rr)
			rep <- rr
			mu.Lock()
			n--
			producer.Signal()
			mu.Unlock()
		}(f)
	}
}

func (r *Reducer) ReadFiles(rtot *readResult) error {
	const MAXCONCURRENCY = 1

	req := make(chan string, r.nmaptask)
	rep := make(chan readResult)

	if MAXCONCURRENCY > 1 {
		go r.readerMgr(req, rep, MAXCONCURRENCY)
	}

	dr := fslib.NewDirReader(r.FsLib, r.input)
	for nfile := 0; nfile < r.nmaptask; {
		files, err := dr.WatchNewUniqueEntries()
		if err != nil {
			return err
		}
		randOffset := int(rand.Uint64())
		if randOffset < 0 {
			randOffset *= -1
		}
		for i, _ := range files {
			nfile += 1
			// Random offset to stop reducer procs from all banging on the same ux.
			f := files[(i+randOffset)%len(files)]
			if MAXCONCURRENCY > 1 {
				req <- f
			} else {
				rr := &readResult{f: f, kvm: rtot.kvm}
				r.readFile(rr)
				rtot.sum(rr)
			}
		}
	}
	if MAXCONCURRENCY > 1 {
		close(req)
		for i := 0; i < r.nmaptask; i++ {
			rr := <-rep
			rtot.sum(&rr)
			rtot.kvm.merge(rr.kvm, r.reducef)
		}
	}
	return nil
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
	db.DPrintf(db.ALWAYS, "DoReduce in %v out %v nmap %v\n", r.input, r.outlink, r.nmaptask)
	rtot := readResult{
		kvm:        newKvmap(MINCAP, MAXCAP),
		mapsFailed: []string{},
	}
	if err := r.ReadFiles(&rtot); err != nil {
		db.DPrintf(db.ALWAYS, "ReadFiles: err %v", err)
		return proc.NewStatusErr(fmt.Sprintf("%v: ReadFiles %v err %v\n", r.ProcEnv().GetPID(), r.input, err), nil)
	}
	if len(rtot.mapsFailed) > 0 {
		return proc.NewStatusErr(RESTART, rtot.mapsFailed)
	}

	ms := rtot.d.Milliseconds()
	db.DPrintf(db.MR, "DoReduce: Readfiles %s: in %s %vms (%s)\n", r.input, humanize.Bytes(uint64(rtot.n)), ms, test.TputStr(rtot.n, ms))

	start := time.Now()

	if err := rtot.kvm.emit(r.reducef, r.emit); err != nil {
		db.DPrintf(db.ALWAYS, "DoReduce: emit err %v", err)
		return proc.NewStatusErr("reducef", err)
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
	rtot.d += time.Since(start)

	// Create symlink atomically.
	if err := r.PutFileAtomic(r.outlink, 0777|sp.DMSYMLINK, []byte(r.tmp)); err != nil {
		return proc.NewStatusErr(fmt.Sprintf("%v: put symlink %v -> %v err %v\n", r.ProcEnv().GetPID(), r.outlink, r.tmp, err), nil)
	}
	return proc.NewStatusInfo(proc.StatusOK, r.input,
		Result{false, r.input, rtot.n, nbyte, rtot.d.Milliseconds()})
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
