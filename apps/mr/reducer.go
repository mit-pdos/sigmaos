package mr

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	"sigmaos/proxy/getput"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	"sigmaos/util/rand"
	"sigmaos/util/tput"
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
	wrt          getput.ShardWriter
	perf         *perf.Perf
	useGetPut    bool
	useCosandbox bool
	clnts        *getput.Clnts
	// Attributes this proc's CPU to its phases; see perf.CPUPhases.
	cpu *perf.CPUPhases
	// Time spent fetching this reducer's input shards; see getStats.
	gets getStats

	// UX servers we have already mounted from the endpoints the coordinator
	// cached for us, keyed by server pathname. readFile may run concurrently.
	mu      sync.Mutex
	uxMnted map[string]bool
}

// cpu, if non-nil, is the proc's CPU-phase chain (see perf.CPUPhases); the
// reducer marks its setup steps on it. Callers that don't care (tests) pass nil.
func NewReducer(sc *sigmaclnt.SigmaClnt, reducef mr.ReduceT, args []string, p *perf.Perf, cpu *perf.CPUPhases) (*Reducer, error) {
	if len(args) != 7 {
		return nil, fmt.Errorf("NewReducer: wrong number of arguments: got %d, want 7 (stale mr-r binary?): %v", len(args), args)
	}
	useGetPut, err := strconv.ParseBool(args[5])
	if err != nil {
		return nil, fmt.Errorf("Reducer: useGetPut %v isn't bool", args[5])
	}
	useCosandbox, err := strconv.ParseBool(args[6])
	if err != nil {
		return nil, fmt.Errorf("Reducer: useCosandbox %v isn't bool", args[6])
	}
	r := &Reducer{
		outlink:      args[2],
		outputTarget: args[3],
		reducef:      reducef,
		SigmaClnt:    sc,
		perf:         p,
		uxMnted:      make(map[string]bool),
		useGetPut:    useGetPut,
		useCosandbox: useCosandbox,
		cpu:          cpu,
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("Reducer: id %v isn't int %v", args[0], err)
	}
	srvId := fttask.FtTaskSvcId(args[1])

	ftclnt := fttask_clnt.NewFtTaskClnt[TreduceTask, Bin](sc.FsLib, srvId, sp.NullFence())

	start := time.Now()
	data, err := ftclnt.ReadTasks([]fttask_clnt.TaskId{fttask_clnt.TaskId(id)})
	if err != nil {
		return nil, fmt.Errorf("Reducer: ReadTasks %v err %v", id, err)
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("Reducer: ReadTasks %v len %d != 1", id, len(data))
	}
	db.DPrintf(db.MR_COORD, "Reducer: ReadTasks %v %v in %v", id, len(data), time.Since(start))
	perf.LogSpawnLatency("Reducer.ReadTasks", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	// Reading this reducer's task: the bin of mapper output shards it reads,
	// which with many mappers is a multi-MiB (compressed) fttask RPC.
	r.cpu.Mark("Reducer.ReadTasks")
	r.input = data[0].Data.Input
	r.tmp = r.outputTarget + rand.Name()

	db.DPrintf(db.MR, "Reducer outputting to %v", r.tmp)

	m, err := strconv.Atoi(args[4])
	if err != nil {
		return nil, fmt.Errorf("Reducer: nmaptask %v isn't int", args[4])
	}
	r.nmaptask = m

	if len(r.input) < 1 {
		return nil, fmt.Errorf("Reducer: input missing %v", r)
	}

	if r.useGetPut {
		r.clnts = getput.NewClnts(sc.FsLib)
	} else if sp.IsS3Path(r.input[0].File) {
		// On the getput path S3 is reached through the local proxy instead, so
		// the S3 path client isn't needed.
		r.MountS3PathClnt()
	}
	r.cpu.Mark("Reducer.mountS3")

	if err := r.initOutput(); err != nil {
		return nil, err
	}
	// Creating the output file.
	r.cpu.Mark("Reducer.initOutput")
	return r, nil
}

// initOutput opens this reducer's output writer. Always the ordinary
// buffered-writer path: get/put is a property of how a reducer *reads* the
// mappers' shards, not of how it writes its output, so a getput or cosandbox
// reducer writes exactly like an fslib one and the configurations differ only
// on the read side.
func (r *Reducer) initOutput() error {
	start := time.Now()
	defer func() {
		perf.LogSpawnLatency("Reducer.initOutput", r.ProcEnv().GetPID(), r.ProcEnv().GetSpawnTime(), start)
	}()
	w, err := r.CreateBufWriter(r.tmp, 0777)
	if err != nil {
		db.DFatalf("Error CreateBufWriter [%v] %v", r.tmp, err)
		return err
	}
	r.wrt = w
	r.pwrt = perf.NewPerfWriter(w, r.perf)
	return nil
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

// mountUxSrv mounts the UX server holding pn from the endpoint the
// coordinator cached for us, so that reading pn doesn't have to find the
// server through named. A reducer reads from one UX server per mapper, so this
// is done lazily, once per server: the attach count stays what it would have
// been. Best-effort; a server we can't mount here is found by walking, as
// before.
//
// There is no S3 equivalent: S3 intermediate output keeps the ~local that
// Mapper.outputBin left in place (it deliberately doesn't resolve S3 paths),
// and sp.S3ClientPath — called in readFile, before we are — rewrites that to
// the s3clnt path client, which talks to S3 directly, with no endpoint
// involved. The reducer's own output goes to ~any, which we don't mount.
func (r *Reducer) mountUxSrv(pn string) {
	rest, ok := strings.CutPrefix(pn, sp.UX)
	if !ok {
		return
	}
	kid := strings.SplitN(rest, "/", 2)[0]
	// Union elements (~local/~any) don't name a server we have an endpoint
	// for; mapper output paths are concrete (Mapper.outputBin resolves them).
	if kid == "" || strings.HasPrefix(kid, "~") {
		return
	}
	srvpn := filepath.Join(sp.UX, kid)

	r.mu.Lock()
	if r.uxMnted[srvpn] {
		r.mu.Unlock()
		return
	}
	r.uxMnted[srvpn] = true
	r.mu.Unlock()

	if _, err := procclnt.MountCachedEndpoint(r.FsLib, srvpn); err != nil {
		db.DPrintf(db.MR, "Reducer MountCachedEndpoint %v err %v", srvpn, err)
	}
}

// openInput opens the idx'th shard of this reducer's input. idx doubles as the
// delegated-RPC index when a cosandbox prefetched the shards: the
// coordinator's manifest issues one whole-file get per shard, in bin order.
func (r *Reducer) openInput(f string, idx int) (getput.FileReader, error) {
	if r.useGetPut {
		return getput.NewGetPutFileReader(r.clnts, f, r.useCosandbox, uint64(idx))
	}
	return r.OpenBufReader(f)
}

func (r *Reducer) readFile(rr *readResult, idx int) {
	if !r.useGetPut {
		// The getput path reaches S3 through the local proxy, so it keeps the
		// sigma pathname; only the fslib path rewrites it to the s3clnt form.
		if pn, ok := sp.S3ClientPath(rr.f); ok {
			rr.f = pn
		}
		r.mountUxSrv(rr.f)
	}
	getStart := time.Now()
	rdr, err := r.openInput(rr.f, idx)
	if err != nil {
		db.DPrintf(db.MR, "NewReader %v err %v", rr.f, err)
		rr.ok = false
		return
	}
	if r.useGetPut {
		// On the getput path the whole shard is fetched at open — a delegated
		// get of what the cosandbox prefetched, or one direct RPC.
		r.gets.record(getStart)
	}
	defer rdr.Close()
	start := time.Now()
	err = ReadKVs(rdr, rr.kvm, r.reducef)
	if !r.useGetPut {
		// On the fslib path the shard streams in as ReadKVs consumes it, so
		// the read cost is in here rather than at open.
		r.gets.record(getStart)
	}
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
			r.readFile(&rr, r.inputIdx(f))
			rep <- rr
			mu.Lock()
			n--
			producer.Signal()
			mu.Unlock()
		}(f)
	}
}

// inputIdx recovers a shard's index in the reduce task's bin, which is the
// rpcIdx its prefetched reply was deposited at. Only the concurrent reader path
// needs it, having passed the pathname through a channel.
func (r *Reducer) inputIdx(f string) int {
	for i := range r.input {
		if r.input[i].File == f {
			return i
		}
	}
	return -1
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
	if r.useCosandbox {
		// Read in bin order instead: the cosandbox issues its prefetches in
		// that order, and a delegated get blocks until its shard's reply
		// materializes, so starting elsewhere would wait on a shard the
		// cosandbox hasn't reached yet while earlier replies pile up. The
		// cosandbox, not this proc, is what spreads the load across the UX
		// servers here.
		randOffset = 0
	}
	for i := 0; i < r.nmaptask; i++ {
		f := (i + randOffset) % r.nmaptask
		if MAXCONCURRENCY > 1 {
			req <- r.input[f].File
		} else {
			rr := &readResult{f: r.input[f].File, kvm: rtot.kvm}
			r.readFile(rr, f)
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
	readStart := time.Now()
	if err := r.ReadFiles(&rtot); err != nil {
		db.DPrintf(db.ALWAYS, "ReadFiles: err %v", err)
		return proc.NewStatusErr(fmt.Sprintf("%v: ReadFiles %v err %v\n", r.ProcEnv().GetPID(), r.input, err), nil)
	}
	perf.LogSpawnLatency("Reducer.ReadFiles", r.ProcEnv().GetPID(), r.ProcEnv().GetSpawnTime(), readStart)
	db.DPrintf(db.MR, "Reducer gets: n %v tot %v", r.gets.count(), r.gets.dur())
	// Reading every mapper's shard for this partition, and combining as they
	// come in (ReadKVs folds each shard into the KV map).
	r.cpu.Mark("Reducer.ReadFiles")
	if len(rtot.mapsFailed) > 0 {
		return proc.NewStatusErr(RESTART, rtot.mapsFailed)
	}

	ms := rtot.d.Milliseconds()
	db.DPrintf(db.MR, "DoReduce: Readfiles %v: in %s %vms (%s)\n", len(r.input), humanize.Bytes(uint64(rtot.n)), ms, tput.TputStr(rtot.n, ms))

	start := time.Now()

	if err := rtot.kvm.Emit(r.reducef, r.emit); err != nil {
		db.DPrintf(db.ALWAYS, "DoReduce: emit err %v", err)
		return proc.NewStatusErr("reducef", err)
	}
	perf.LogSpawnLatency("Reducer.emit", r.ProcEnv().GetPID(), r.ProcEnv().GetSpawnTime(), start)
	// Running the reduce function over the combined KV map and writing the
	// results out.
	r.cpu.Mark("Reducer.emit")

	closeStart := time.Now()
	if err := r.wrt.Close(); err != nil {
		return proc.NewStatusErr(fmt.Sprintf("%v: close %v err %v\n", r.ProcEnv().GetPID(), r.tmp, err), nil)
	}
	nbyte := r.wrt.Nbytes()
	perf.LogSpawnLatency("Reducer.closeWrt", r.ProcEnv().GetPID(), r.ProcEnv().GetSpawnTime(), closeStart)
	// Flushing the output: on the getput path an S3 target's whole object is
	// uploaded here.
	r.cpu.Mark("Reducer.closeWrt")

	// Include time spent writing output.
	rtot.d += time.Since(start)

	// Create symlink atomically. Retry on version issues
	linkStart := time.Now()
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
	perf.LogSpawnLatency("Reducer.putOutlink", r.ProcEnv().GetPID(), r.ProcEnv().GetSpawnTime(), linkStart)
	// Publishing the output: the symlink the job's output directory points at.
	r.cpu.Mark("Reducer.putOutlink")
	return proc.NewStatusInfo(proc.StatusOK, "OK",
		Result{
			IsM:      false,
			Task:     r.ProcEnv().GetPID().String(),
			In:       rtot.n,
			Out:      nbyte,
			OutBin:   Bin{},
			MsInner:  rtot.d.Milliseconds(),
			MsGet:    r.gets.dur().Milliseconds(),
			NGet:     r.gets.count(),
			KernelID: r.ProcEnv().GetKernelID(),
		})
}

func RunReducer(reducef mr.ReduceT, args []string) {
	pe := proc.GetProcEnv()
	// Split the reducer's CPU into setup (sigmaclnt, reading its task, opening
	// its output) and the reduce itself, to compare with the mapper's split.
	cpu := perf.NewCPUPhases(pe)
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
	// Building the SigmaClnt: parsing the ProcEnv, and mounting named and
	// msched (in parallel) from the endpoints procd cached for us.
	cpu.Mark("Reducer.NewSigmaClnt")
	if err := sc.Started(); err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	// Notifying msched that we started.
	cpu.Mark("Reducer.Started")
	r, err := NewReducer(sc, reducef, args, p, cpu)
	if err != nil {
		db.DPrintf(db.ERROR, "Err reducer: %v", err)
		r.ClntExit(proc.NewStatusErr("NewReducer err", err))
	}
	crash.Failer(sc.FsLib, crash.MRREDUCE_CRASH, func(e crash.Tevent) {
		crash.Crash()
	})
	crash.Failer(sc.FsLib, crash.MRREDUCE_PARTITION, func(e crash.Tevent) {
		crash.PartitionPath(sc.FsLib, r.input[0].File)
	})
	// Whatever setup is left: installing the crash failers.
	cpu.Mark("Reducer.setupTail")
	status := r.DoReduce()
	// Whatever is left between DoReduce returning and ClntExit (which reports
	// Proc.exit.CPU, i.e. the total).
	cpu.Mark("Reducer.postDoReduce")
	r.ClntExit(status)
}
