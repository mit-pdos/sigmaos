package mr

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"sigmaos/apps/mr/chunkreader"
	"sigmaos/apps/mr/kvmap"
	"sigmaos/apps/mr/mr"
	db "sigmaos/debug"
	fttask "sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	"sigmaos/util/rand"
)

const (
	DEFAULT_KEY_BUF_SZ = 1000
	DEFAULT_VAL_BUF_SZ = 10000
)

type Reducer struct {
	*sigmaclnt.SigmaClnt
	reducef      mr.ReduceT
	input        Bin
	outputTarget string
	outlink      string
	nmaptask     int
	tmp          string
	pwrt         *perf.PerfWriter
	wrt          *fslib.FileWriter
	perf         *perf.Perf
}

func NewReducer(sc *sigmaclnt.SigmaClnt, reducef mr.ReduceT, args []string, p *perf.Perf) (*Reducer, error) {
	r := &Reducer{
		outlink:      args[2],
		outputTarget: args[3],
		reducef:      reducef,
		SigmaClnt:    sc,
		perf:         p,
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("Reducer: id %v isn't int %v", args[0], err)
	}
	srvId := fttask.FtTaskSrvId(args[1])

	ftclnt := fttask_clnt.NewFtTaskClnt[TreduceTask, Bin](sc.FsLib, srvId)

	start := time.Now()
	data, err := ftclnt.ReadTasks([]fttask_clnt.TaskId{fttask_clnt.TaskId(id)})
	if err != nil {
		return nil, fmt.Errorf("Reducer: ReadTasks %v err %v", id, err)
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("Reducer: ReadTasks %v len %d != 1", id, len(data))
	}
	db.DPrintf(db.MR_COORD, "Reducer: ReadTasks %v %v in %v", id, len(data), time.Since(start))
	r.input = data[0].Data.Input
	r.tmp = r.outputTarget + rand.Name()

	db.DPrintf(db.MR, "Reducer outputting to %v", r.tmp)

	m, err := strconv.Atoi(args[4])
	if err != nil {
		return nil, fmt.Errorf("Reducer: nmaptask %v isn't int", args[4])
	}
	r.nmaptask = m

	if sp.IsS3Path(r.input[0].File) {
		r.MountS3PathClnt()
	}

	w, err := r.CreateBufWriter(r.tmp, 0777)
	if err != nil {
		db.DFatalf("Error CreateBufWriter [%v] %v", r.tmp, err)
		return nil, err
	}
	r.wrt = w
	r.pwrt = perf.NewPerfWriter(r.wrt, r.perf)
	return r, nil
}

func ReadKVs(rdr io.Reader, kvm *kvmap.KVMap, reducef mr.ReduceT) error {
	kvd := newKVDecoder(rdr, DEFAULT_KEY_BUF_SZ, DEFAULT_VAL_BUF_SZ)
	for {
		if k, v, err := kvd.decode(); err != nil {
			if err == io.EOF {
				break
			}
			if serr.IsErrorSession(err) {
				return err
			}
		} else {
			if err := kvm.Combine(k, v, reducef); err != nil {
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
	kvm        *kvmap.KVMap
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
	pn, ok := sp.S3ClientPath(rr.f)
	if ok {
		rr.f = pn
	}
	rdr, err := r.OpenBufReader(rr.f)
	if err != nil {
		db.DPrintf(db.MR, "NewReader %v err %v", rr.f, err)
		rr.ok = false
		return
	}
	defer rdr.Close()
	start := time.Now()
	err = ReadKVs(rdr, rr.kvm, r.reducef)
	db.DPrintf(db.MR, "Reduce readfile %v %dms err %v\n", rr.f, time.Since(start).Milliseconds(), err)
	if err != nil {
		db.DPrintf(db.MR, "decodeKV %v err %v\n", rr.f, err)
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
			kvm := kvmap.NewKVMap(chunkreader.MINCAP, chunkreader.MAXCAP)
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

	// Random offset to stop reducer procs from all banging on the same ux.
	randOffset := int(rand.Uint64())
	if randOffset < 0 {
		randOffset *= -1
	}
	for i := 0; i < r.nmaptask; i++ {
		f := (i + randOffset) % r.nmaptask
		if MAXCONCURRENCY > 1 {
			req <- r.input[f].File
		} else {
			rr := &readResult{f: r.input[f].File, kvm: rtot.kvm}
			r.readFile(rr)
			rtot.sum(rr)
		}
	}
	if MAXCONCURRENCY > 1 {
		close(req)
		for i := 0; i < r.nmaptask; i++ {
			rr := <-rep
			rtot.sum(&rr)
			rtot.kvm.Merge(rr.kvm, r.reducef)
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
	db.DPrintf(db.ALWAYS, "DoReduce in %v out %v nmap %v\n", len(r.input), r.outlink, r.nmaptask)
	rtot := readResult{
		kvm:        kvmap.NewKVMap(chunkreader.MINCAP, chunkreader.MAXCAP),
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
	db.DPrintf(db.MR, "DoReduce: Readfiles %v: in %s %vms (%s)\n", len(r.input), humanize.Bytes(uint64(rtot.n)), ms, test.TputStr(rtot.n, ms))

	start := time.Now()

	if err := rtot.kvm.Emit(r.reducef, r.emit); err != nil {
		db.DPrintf(db.ALWAYS, "DoReduce: emit err %v", err)
		return proc.NewStatusErr("reducef", err)
	}

	if err := r.wrt.Close(); err != nil {
		return proc.NewStatusErr(fmt.Sprintf("%v: close %v err %v\n", r.ProcEnv().GetPID(), r.tmp, err), nil)
	}
	nbyte := r.wrt.Nbytes()

	// Include time spent writing output.
	rtot.d += time.Since(start)

	// Create symlink atomically. Retry on version issues
	for {
		if err := r.PutFileAtomic(r.outlink, 0777|sp.DMSYMLINK, []byte(r.tmp)); err != nil {
			if se, ok := serr.IsErr(err); ok && se.IsErrVersion() {
				db.DPrintf(db.MR, "Version err PutFileAtomic: retrying")
				continue
			}
			return proc.NewStatusErr(fmt.Sprintf("%v: put symlink %v -> %v err %v\n", r.ProcEnv().GetPID(), r.outlink, r.tmp, err), nil)
		}
		break
	}
	return proc.NewStatusInfo(proc.StatusOK, "OK",
		Result{false, r.ProcEnv().GetPID().String(), rtot.n, nbyte, Bin{}, rtot.d.Milliseconds(), 0, r.ProcEnv().GetKernelID()})
}

func RunReducer(reducef mr.ReduceT, args []string) {
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
	if err := r.Started(); err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	crash.Failer(sc.FsLib, crash.MRREDUCE_CRASH, func(e crash.Tevent) {
		crash.Crash()
	})
	crash.Failer(sc.FsLib, crash.MRREDUCE_PARTITION, func(e crash.Tevent) {
		crash.PartitionPath(sc.FsLib, r.input[0].File)
	})
	status := r.DoReduce()
	r.ClntExit(status)
}
