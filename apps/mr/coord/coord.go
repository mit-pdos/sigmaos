// The coord package implements the MapReduce coordinator: it hands map and
// reduce tasks to mapper/reducer procs (sigmaos/apps/mr) and tracks their
// results.
//
// It is a separate package from sigmaos/apps/mr so that mapper and reducer
// binaries don't link what only the coordinator needs — the fttask server
// (etcd + gRPC), the WASM runtime (cgo libwasmer), and the test harness (the
// Docker client). Package init of those costs every proc that links them
// >10ms.
package coord

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/ft/task"
	ftclnt "sigmaos/ft/task/clnt"
	"sigmaos/ft/task/fttaskmgr"
	ftmgr "sigmaos/ft/task/fttaskmgr"
	"sigmaos/proc"
	wasmer "sigmaos/proxy/wasm/rpc/wasmer"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	"sigmaos/util/rand"
	"sigmaos/util/spstats"
)

const (
	NCOORD               = 1
	MALICIOUS_MAPPER_BIN = "mr-m-malicious"

	// Floor on the shared-memory segment sized by mapperShmemMB, so that a job
	// with tiny splits still leaves the allocator room for reply framing.
	MIN_SHMEM_MB proc.Tmem = 2
)

// mr_test puts pathnames of input files (split into bins) in
// MR/<job>/m/ and creates an fttask task for each one of them.  It
// also creates a number of reducer tasks (one for each reducer).
//
// The coordinator claims tasks and start procs for them, which
// process the claimed task.  Mapper i creates <r> output shards, one
// for each reducer and returns a bin of pathnames for the shards to
// the coordinator.
//
// If a mapper or reducer proc successfully exits, the coordinator
// marks the task as done and stores the pathnames returned by the
// mapper with the task.  If it fails, the coordinator will make the
// task runnable again and start new mapper/reducer procs to process
// the task.  If the coordinator fails, another coordinator will take
// over and claim tasks.

type Coord struct {
	*sigmaclnt.SigmaClnt
	mftid            task.FtTaskSvcId
	rftid            task.FtTaskSvcId
	mftclnt          ftclnt.FtTaskClnt[mr.Bin, mr.Bin]
	rftclnt          ftclnt.FtTaskClnt[mr.TreduceTask, mr.Bin]
	mcoord           *fttaskmgr.FtTaskCoord[[]byte, []byte]
	rcoord           *fttaskmgr.FtTaskCoord[[]byte, []byte]
	jobRoot          string
	job              string
	nmaptask         int
	nreducetask      int
	maliciousMapper  uint64
	linesz           string
	lineszInt        int
	wordsz           string
	mapperbin        string
	reducerbin       string
	leaderclnt       *leaderclnt.LeaderClnt
	outdir           string
	intOutdir        string
	memPerTask       proc.Tmem
	stat             AStat
	perf             *perf.Perf
	useGetPut        bool
	useCosandbox     bool
	tailProbeSz      int
	binsz            int
	mrBootWASM       []byte
	uxEPs            *procclnt.SrvEPCache
	s3EPs            *procclnt.SrvEPCache
	intOutS3         bool
	mapperGOMAXPROCS int
	phaseStart       time.Time
	mapPhaseMs       int64
	mapPhaseDone     bool
}

type AStat struct {
	Ntask          spstats.Tcounter
	Nmap           spstats.Tcounter
	Nreduce        spstats.Tcounter
	Nfail          spstats.Tcounter
	Nrestart       spstats.Tcounter
	NrecoverMap    spstats.Tcounter
	NrecoverReduce spstats.Tcounter
}

func (s *AStat) String() string {
	return fmt.Sprintf("{nT %d nM %d nR %d nfail %d nrestart %d nrecoverM %d nrecoverR %d}", s.Ntask.Load(), s.Nmap.Load(), s.Nreduce.Load(), s.Nfail.Load(), s.Nrestart.Load(), s.NrecoverMap.Load(), s.NrecoverReduce.Load())
}

type NewProc func(ftclnt.Task[[]byte]) (*proc.Proc, error)

func NewCoord(args []string) (*Coord, error) {
	if len(args) != 17 {
		return nil, fmt.Errorf("NewCoord: wrong number of arguments: got %d, want 17 (stale mr-coord binary?): %v", len(args), args)
	}
	c := &Coord{}
	c.jobRoot = args[1]
	c.job = args[0]
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	perf, _ := perf.NewPerf(proc.GetProcEnv(), perf.MRCOORD)
	c.perf = perf
	db.DPrintf(db.MR_COORD, "Made fslib job %v", c.job)
	c.SigmaClnt = sc
	m, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nmaptask %v isn't int", args[2])
	}
	n, err := strconv.Atoi(args[3])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nreducetask %v isn't int", args[3])
	}
	c.nmaptask = m
	c.nreducetask = n

	c.mapperbin = args[4]
	c.reducerbin = args[5]

	malmap, err := strconv.Atoi(args[9])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: maliciousMapper %v isn't int", args[9])
	}
	c.maliciousMapper = uint64(malmap)

	c.linesz = args[6]
	c.wordsz = args[7]

	mem, err := strconv.Atoi(args[8])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: nreducetask %v isn't int", args[3])
	}
	c.memPerTask = proc.Tmem(mem)

	b, err := c.GetFile(mr.JobOutLink(c.jobRoot, c.job))
	if err != nil {
		db.DFatalf("Error GetFile JobOutLink [%v]: %v", mr.JobOutLink(c.jobRoot, c.job), err)
	}
	c.outdir = string(b)

	b, err = c.GetFile(mr.JobIntOutLink(c.jobRoot, c.job))
	if err != nil {
		db.DFatalf("Error GetFile JobIntOutLink: %v", err)
	}
	c.intOutdir = string(b)

	c.Started()

	c.leaderclnt, err = leaderclnt.NewLeaderClnt(c.FsLib, mr.LeaderElectDir(c.job)+"/coord-leader", 0)
	if err != nil {
		return nil, fmt.Errorf("NewCoord: NewLeaderclnt err %v", err)
	}

	c.mftid = task.FtTaskSvcId(args[10])
	c.rftid = task.FtTaskSvcId(args[11])

	c.useGetPut, err = strconv.ParseBool(args[12])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: useGetPut %v isn't bool", args[12])
	}
	c.useCosandbox, err = strconv.ParseBool(args[13])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: useCosandbox %v isn't bool", args[13])
	}
	c.tailProbeSz, err = strconv.Atoi(args[14])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: tailprobesz %v isn't int", args[14])
	}
	c.lineszInt, err = strconv.Atoi(c.linesz)
	if err != nil {
		return nil, fmt.Errorf("NewCoord: linesz %v isn't int", c.linesz)
	}
	c.mapperGOMAXPROCS, err = strconv.Atoi(args[15])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: mapperGOMAXPROCS %v isn't int", args[15])
	}
	c.binsz, err = strconv.Atoi(args[16])
	if err != nil {
		return nil, fmt.Errorf("NewCoord: binsz %v isn't int", args[16])
	}

	if c.useCosandbox {
		// Read and precompile the mapper boot script once; every mapper
		// proc gets the same compiled WASM with a per-bin manifest.
		c.mrBootWASM, err = wasmer.ReadCoSandbox(c.SigmaClnt, "mr_mapper_boot")
		if err != nil {
			return nil, fmt.Errorf("NewCoord: ReadCoSandbox mr_mapper_boot err %v", err)
		}
	}

	// Learn the endpoints of the servers this job's procs will use, once
	// here, so that each mapper/reducer can mount the ones it needs without
	// walking the namespace to find them.
	c.intOutS3 = strings.HasPrefix(c.intOutdir, sp.S3)
	if strings.HasPrefix(c.intOutdir, sp.UX) {
		c.uxEPs = c.newSrvEPCache(sp.UX)
	}
	// The local S3 proxy is only reached on the getput path. Mappers on the
	// fslib path don't need its endpoint — sp.S3ClientPath rewrites
	// name/s3/~local to the s3clnt path client, which talks to S3 directly —
	// and neither do reducers (see reducerProc).
	if c.useGetPut {
		c.s3EPs = c.newSrvEPCache(sp.S3)
	}

	return c, nil
}

// Create an endpoint cache for the servers under unionpn and warm it here,
// off the task-execution path, so that mapperProc/reducerProc never block on
// named. Discovery failures aren't fatal: children fall back to walking.
func (c *Coord) newSrvEPCache(unionpn string) *procclnt.SrvEPCache {
	epc := procclnt.NewSrvEPCache(c.FsLib, unionpn)
	start := time.Now()
	if eps, err := epc.Endpoints(); err != nil {
		db.DPrintf(db.ALWAYS, "EP cache %v: discovery failed (%v); procs will walk the namespace instead", unionpn, err)
	} else if len(eps) == 0 {
		db.DPrintf(db.ALWAYS, "EP cache %v: no servers found; procs will walk the namespace instead", unionpn)
	} else {
		db.DPrintf(db.MR_COORD, "EP cache %v warmed in %v: %v", unionpn, time.Since(start), epc.Srvs())
	}
	return epc
}

// Hand a set of servers' endpoints to a proc we are about to spawn, so that
// it can mount the ones it uses directly. Best-effort: a proc that doesn't
// get them (or gets a stale one) walks the namespace as it did before.
func (c *Coord) cacheEPs(p *proc.Proc, epc *procclnt.SrvEPCache) {
	if epc == nil {
		return
	}
	if err := epc.CacheEndpoints(p); err != nil {
		db.DPrintf(db.MR_COORD, "cacheEPs %v %v err %v", epc, p.GetPid(), err)
		return
	}
	db.DPrintf(db.MR_COORD, "cacheEPs %v -> %v", epc, p.GetPid())
}

// The splits in a bin all name files in the same input directory (NewBins),
// so the first one tells us where a mapper's input lives.
func binUsesS3(bin mr.Bin) bool {
	return len(bin) > 0 && sp.IsS3Path(bin[0].File)
}

func (c *Coord) newTask(bin string, args []string, mb proc.Tmem) *proc.Proc {
	pid := sp.GenPid(bin + "-" + c.job)
	p := proc.NewProcPid(pid, bin, args)
	//	if mb > 0 {
	//		p.AppendEnv("GOMEMLIMIT", strconv.Itoa(int(mb)*1024*1024))
	//	}
	p.SetMem(mb)
	return p
}

func (c *Coord) mapperProc(t ftclnt.Task[[]byte]) (*proc.Proc, error) {
	bin, err := ftclnt.Decode[mr.Bin](t.Data)
	if err != nil {
		db.DFatalf("mapperProc: failed to convert data to bin %v %v", t.Data, err)
	}

	mapperbin := c.mapperbin
	// If running with malicious mappers, roll the dice and see if we should
	// spawn a benign mapper or a malicious one.
	if c.maliciousMapper > 0 {
		roll := rand.Int64(1000)
		if roll < c.maliciousMapper {
			// Roll successful: switch to malicious mapper
			mapperbin = MALICIOUS_MAPPER_BIN
		}
	}
	c.stat.Nmap.Add(1)

	b, err := json.Marshal(bin)
	if err != nil {
		db.DFatalf("mapperProc: %v err %v", bin, err)
	}
	p := c.newTask(mapperbin, []string{c.jobRoot, c.job, strconv.Itoa(c.nreducetask), string(b), c.intOutdir, c.linesz, c.wordsz, strconv.FormatBool(c.useGetPut), strconv.FormatBool(c.useCosandbox), strconv.Itoa(c.tailProbeSz)}, c.memPerTask)
	if c.mapperGOMAXPROCS > 0 {
		// Bound the mapper's Go runtime instead of letting it size itself to
		// the whole machine, which every proc sharing the machine otherwise
		// does independently.
		p.AppendEnv("GOMAXPROCS", strconv.Itoa(c.mapperGOMAXPROCS))
	}
	if c.useGetPut {
		// The UX/S3 proxy client RPC channels — and the delegated-RPC path
		// in particular — are serviced by spproxy.
		p.GetProcEnv().UseSPProxy = true
	}
	if c.useCosandbox {
		input, err := mapperBootInput(bin, c.lineszInt, c.tailProbeSz)
		if err != nil {
			return nil, err
		}
		// Retrieve the cosandbox's prefetched splits through shared memory
		// rather than copying them back over the spproxy socket.
		shmemMB := mapperShmemMB(c.binsz)
		p.SetShmemMB(shmemMB)
		db.DPrintf(db.MR_COORD, "mapperProc %v cosandbox shmem %vMB", p.GetPid(), shmemMB)
		p.SetCoSandbox(c.mrBootWASM, input)
		p.SetRunCoSandbox(true)
		// Deliberately no SetRunAfterCoSandbox(true): DelegatedRPC blocks
		// until the reply for each rpcIdx materializes, so the mapper
		// starts immediately and pipelines against in-flight prefetches.
	}
	c.cacheEPs(p, c.uxEPs)
	// A mapper reaches S3 through the local S3 proxy only on the getput path:
	// for its input, when the bin is in S3, and for its output, when the
	// intermediate directory is.
	if c.useGetPut && (c.intOutS3 || binUsesS3(bin)) {
		c.cacheEPs(p, c.s3EPs)
	}
	return p, nil
}

func (c *Coord) reducerProc(t ftclnt.Task[[]byte]) (*proc.Proc, error) {
	data, err := ftclnt.Decode[mr.TreduceTask](t.Data)
	if err != nil {
		db.DFatalf("reducerProc: failed to convert data to task %v %v", t.Data, err)
	}
	outlink := mr.ReduceOut(c.jobRoot, c.job) + data.Task
	outTarget := mr.ReduceOutTarget(c.outdir, c.job) + data.Task
	c.stat.Nreduce.Add(1)
	p := c.newTask(c.reducerbin, []string{strconv.Itoa(int(t.Id)), string(c.rftclnt.ServiceId()), outlink, outTarget, strconv.Itoa(c.nmaptask)}, c.memPerTask)
	// Only UX: a reducer reading UX intermediate output walks to a concrete
	// name/ux/<kid> per mapper (Mapper.outputBin resolved ~local for them),
	// but S3 intermediate output keeps its ~local (outputBin deliberately
	// doesn't resolve S3 paths), which sp.S3ClientPath rewrites to the s3clnt
	// path client — no endpoint involved. The reducer's own output goes
	// through ~any, which we deliberately don't mount.
	c.cacheEPs(p, c.uxEPs)
	return p, nil
}

func newStringSlice(data []interface{}) []string {
	s := make([]string, 0, len(data))
	for _, o := range data {
		s = append(s, o.(string))
	}
	return s
}

// A reducer failed because it couldn't read its input file; we must
// restart mapper.  We let all mappers and reducers finish, before
// restarting any mappers and reducers, which avoids restarting a
// mapper several times (because several reducers may ask the mapper
// to be restarted).
func (c *Coord) restart(files []string, task ftclnt.TaskId) {
	db.DPrintf(db.ALWAYS, "restart: files %v for %v\n", files, task)
	for _, f := range files {
		if err := c.Remove(f); err != nil {
			db.DPrintf(db.ALWAYS, "remove %v err %v\n", f, err)
		}
	}
	// Record that we have to rerun reducer task
	if err := c.rftclnt.MoveTasks([]ftclnt.TaskId{task}, ftclnt.ERROR); err != nil {
		db.DPrintf(db.ALWAYS, "restart reducer %v err %v\n", task, err)
	}
}

// Mark all errored mapper tasks as runnable. If there are errored
// reducers, mark all mappers as errored.
func (c *Coord) doRestart() {
	start := time.Now()
	// Tasks failed and we are about to respawn them. A server may have
	// restarted with a new endpoint, so re-discover in the background; the
	// procs we spawn meanwhile keep the endpoints we already have. This is
	// hygiene, not correctness: a child that finds a stale endpoint falls
	// back to walking the namespace. The refresh logs its own outcome (and
	// what changed) when it completes.
	for _, epc := range []*procclnt.SrvEPCache{c.uxEPs, c.s3EPs} {
		if epc != nil {
			db.DPrintf(db.MR_COORD, "doRestart: refresh EP cache %v srvs %v", epc, epc.Srvs())
			epc.Refresh()
		}
	}
	ts, err := c.rftclnt.GetTasksByStatus(ftclnt.ERROR)
	if err != nil {
		db.DFatalf("doRestart: move error err %v\n", err)
	}
	if len(ts) > 0 {
		// if a reducer couldn't read its input files, mark all
		// mappers as failed so that they will be restarted.
		_, err := c.mftclnt.MoveTasksByStatus(ftclnt.DONE, ftclnt.ERROR)
		if err != nil {
			db.DFatalf("doRestart: move done err %v\n", err)
		}
	}
	n, err := c.mftclnt.MoveTasksByStatus(ftclnt.ERROR, ftclnt.TODO)
	if err != nil {
		db.DFatalf("doRestart:  mappers error err %v", err)
	}
	m := int32(len(ts))
	if n+m > 0 {
		db.DPrintf(db.ALWAYS, "doRestart: restart %d tasks", n+m)
	}
	spstats.Inc(&c.stat.Nrestart, int64(n+m))
	db.DPrintf(db.MR_COORD, "doRestart took %v", time.Since(start))
}

func (c *Coord) updateReducers(ids []ftclnt.TaskId, bins map[ftclnt.TaskId]mr.Bin) error {
	start := time.Now()
	rtaskData, err := c.rftclnt.ReadTasks(ids)
	if err != nil {
		db.DPrintf(db.MR_COORD, "updateReducers: ReadTasks %v err %v", ids, err)
		return err
	}

	db.DPrintf(db.MR_COORD, "updateReducers: read %v tasks %v", len(rtaskData), time.Since(start))

	for i, t := range ids {
		if reduceBin, ok := bins[t]; ok {
			rtaskData[i].Data.Input = reduceBin
		} else {
			db.DFatalf("updateReducers: no input for %v", t)
		}
	}

	// if we have a lot of mappers and errored reducers, this can be a
	// lot of data, which exceeds the RPC message size limits, so we
	// break the update up into batches
	tasks := make([]*ftclnt.Task[mr.TreduceTask], len(rtaskData))
	for i := range rtaskData {
		tasks[i] = &rtaskData[i]
	}
	start = time.Now()
	if err := ftclnt.BatchTasks(tasks, func(batch []*ftclnt.Task[mr.TreduceTask]) error {
		if _, err := c.rftclnt.EditTasks(batch); err != nil {
			db.DPrintf(db.MR_COORD, "EditTasks batch err %v", err)
		}
		db.DPrintf(db.MR_COORD, "updateReducers: EditTasks %v tasks %v", len(batch), time.Since(start))
		start = time.Now()
		return nil
	}); err != nil {
		return err
	}

	// now all mappers are done and reduce tasks updated, resurrect
	// errored-out reducers, if any
	_, err = c.rftclnt.MoveTasksByStatus(ftclnt.ERROR, ftclnt.TODO)
	if err != nil {
		return err
	}

	return nil
}

func (c *Coord) createReducers(bins map[ftclnt.TaskId]mr.Bin) error {
	tasks := make([]*ftclnt.Task[mr.TreduceTask], c.nreducetask)
	for r := 0; r < c.nreducetask; r++ {
		t := mr.TreduceTask{strconv.Itoa(r), bins[int32(r)]}
		tasks[r] = &ftclnt.Task[mr.TreduceTask]{Id: ftclnt.TaskId(r), Data: t}
	}

	// submitting all reduce tasks at once may exceed the RPC message
	// size limits, so we break the submission up into batches
	totalStart := time.Now()
	start := totalStart
	if err := ftclnt.BatchTasks(tasks, func(batch []*ftclnt.Task[mr.TreduceTask]) error {
		if err := c.rftclnt.SubmitTasks(batch); err != nil {
			db.DPrintf(db.MR_COORD, "Err SubmitTasks: %v", err)
			return err
		}
		db.DPrintf(db.MR_COORD, "createReducers: SubmitTasks %v tasks %v", len(batch), time.Since(start))
		start = time.Now()
		return nil
	}); err != nil {
		return err
	}
	db.DPrintf(db.MR_COORD, "createReducers: %v took %v", c.nreducetask, time.Since(totalStart))
	return nil
}

// Assumes no more mappers and reducers in WIP; that is, all mappers
// are done. Creates all reduce tasks or updates the ones that are
// errored out.
func (c *Coord) makeReduceBins() error {
	mns, err := c.mftclnt.GetTasksByStatus(ftclnt.DONE)
	if err != nil {
		db.DPrintf(db.MR_COORD, "Err GetTasksByStatus(DONE): %v", err)
		return err
	}
	obins, err := c.mftclnt.GetTaskOutputs(mns)
	if err != nil {
		db.DPrintf(db.MR_COORD, "Err GetTaskOutputs: %v", err)
		return err
	}

	db.DPrintf(db.MR_COORD, "makeReduceBins: obins(%d) %v", len(obins), obins)

	rns := make([]ftclnt.TaskId, c.nreducetask)
	reduceBinIn := make(map[ftclnt.TaskId]mr.Bin, c.nreducetask)
	for i, _ := range rns {
		rns[i] = ftclnt.TaskId(i)
		reduceBinIn[rns[i]] = make(mr.Bin, c.nmaptask)
	}
	for j, obin := range obins {
		for i, s := range obin {
			reduceBinIn[rns[i]][j] = s
		}
	}

	db.DPrintf(db.MR_COORD, "makeReduceBins %d: reduceBinIn %v", len(reduceBinIn), reduceBinIn)

	rnsError, err := c.rftclnt.GetTasksByStatus(ftclnt.ERROR)
	if err != nil {
		db.DPrintf(db.MR_COORD, "Err GetTasksByStatus(ERROR): %v", err)
		return err
	}

	if len(rnsError) > 0 {
		return c.updateReducers(rnsError, reduceBinIn)
	} else {
		return c.createReducers(reduceBinIn)
	}

	return nil
}

func (c *Coord) Work() {
	db.DPrintf(db.MR_COORD, "Try acquire leadership coord %v job %v", c.ProcEnv().GetPID(), c.job)

	// Try to become the leading coordinator.
	if err := c.leaderclnt.LeadAndFence(nil, []string{mr.JobDir(c.jobRoot, c.job)}); err != nil {
		db.DFatalf("LeadAndFence err %v", err)
	}

	db.DPrintf(db.ALWAYS, "leader %s nmap %v nreduce %v\n", c.job, c.nmaptask, c.nreducetask)

	f := c.leaderclnt.Fence()

	c.mftclnt = ftclnt.NewFtTaskClnt[mr.Bin, mr.Bin](c.FsLib, c.mftid, &f)
	c.rftclnt = ftclnt.NewFtTaskClnt[mr.TreduceTask, mr.Bin](c.FsLib, c.rftid, &f)

	if err := c.mftclnt.Fence(&f); err != nil {
		db.DFatalf("Fence mapper err %v", err)
	}
	if err := c.rftclnt.Fence(&f); err != nil {
		db.DFatalf("Fence reducer err %v", err)
	}

	var err error
	ch := make(chan ftmgr.Tresult[[]byte, []byte])
	c.mcoord, err = fttaskmgr.NewFtTaskCoord[[]byte, []byte](c.SigmaClnt, c.mftclnt.AsRawClnt(), ch)
	c.rcoord, err = fttaskmgr.NewFtTaskCoord[[]byte, []byte](c.SigmaClnt, c.rftclnt.AsRawClnt(), ch)

	crash.Failer(c.FsLib, crash.MRCOORD_CRASH, func(e crash.Tevent) {
		crash.CrashMsg(c.stat.String())
	})
	crash.Failer(c.FsLib, crash.MRCOORD_PARTITION, func(e crash.Tevent) {
		crash.PartitionNamed(c.FsLib)
	})

	start := time.Now()
	if n, err := c.mftclnt.MoveTasksByStatus(ftclnt.WIP, ftclnt.TODO); err != nil {
		db.DFatalf("RecoverTasks mapper err %v", err)
	} else {
		spstats.Store(&c.stat.NrecoverMap, int64(n))
		db.DPrintf(db.MR_COORD, "Recover %d WIP map tasks took %v", n, time.Since(start))
	}

	start = time.Now()
	if n, err := c.rftclnt.MoveTasksByStatus(ftclnt.WIP, ftclnt.TODO); err != nil {
		db.DFatalf("RecoverTasks reducer err %v", err)
	} else {
		c.stat.NrecoverReduce.Store(int64(n))
		db.DPrintf(db.MR_COORD, "Recover %d WIP reduce tasks took %v", n, time.Since(start))
	}

	c.doRestart()

	m, err := c.mftclnt.GetNTasks(ftclnt.DONE)
	if err != nil {
		db.DFatalf("NtaskDone mappers err %v\n", err)
	}
	r, err := c.rftclnt.GetNTasks(ftclnt.DONE)
	if err != nil {
		db.DFatalf("NtaskDone reducers err %v\n", err)
	}

	start = time.Now()
	c.phaseStart = start
	if int(m+r) < c.nmaptask+c.nreducetask {

		wg := &sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			st, err := c.mcoord.ExecuteTasks(c.mapperProc)
			if err != nil {
				db.DFatalf("ExecuteTasks: mcoord err %v", err)
			}
			n := st.NumDone + st.NumError + st.NumTodo + st.NumWip
			spstats.Inc(&c.stat.Ntask, int64(n))
		}()
		go func() {
			defer wg.Done()
			st, err := c.rcoord.ExecuteTasks(c.reducerProc)
			if err != nil {
				db.DFatalf("ExecuteTasks: rcoord err %v", err)
			}
			n := st.NumDone + st.NumError + st.NumTodo + st.NumWip
			spstats.Inc(&c.stat.Ntask, int64(n))
		}()

		go c.processResult(ch, m, r)

		wg.Wait()
		close(ch)

		// double check we are done
		m, err = c.mftclnt.GetNTasks(ftclnt.DONE)
		if err != nil {
			db.DFatalf("NtaskDone mappers err %v\n", err)
		}
		r, err = c.rftclnt.GetNTasks(ftclnt.DONE)
		if err != nil {
			db.DFatalf("NtaskDone reducers err %v\n", err)
		}
	}

	if int(m+r) < c.nmaptask+c.nreducetask {
		db.DFatalf("job isn't done %v+%v != %v+%v", m, r, c.nmaptask, c.nreducetask)
	}

	db.DPrintf(db.ALWAYS, "job done stat %v", &c.stat)

	db.DPrintf(db.ALWAYS, "E2e bench took %v", time.Since(start))
	mr.JobDone(c.FsLib, c.jobRoot, c.job)

	stro := spstats.NewTcounterSnapshot()
	stro.FillCounters(&c.stat)
	stro.MergeCounters(c.ProcAPI.Stats())
	c.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK", stro))
	defer c.perf.Done()
}

// Record the wall-clock time (measured from the coordinator's start of task
// execution) at which the map phase completed. Called the first time all map
// tasks are done. Note: on recovery/failure the phase boundary isn't corrected
// for.
func (c *Coord) recordMapPhaseDone() {
	if c.mapPhaseDone {
		return
	}
	c.mapPhaseDone = true
	c.mapPhaseMs = time.Since(c.phaseStart).Milliseconds()
	db.DPrintf(db.ALWAYS, "map phase took %vms", c.mapPhaseMs)
}

// Record the wall-clock duration of the reduce phase (time from the end of the
// map phase to the completion of all reduce tasks) and persist both phase
// durations to the job's phase-stats file so the driver can report them.
func (c *Coord) recordReducePhaseDone() {
	totalMs := time.Since(c.phaseStart).Milliseconds()
	pd := &mr.PhaseDurations{
		MapMs:    c.mapPhaseMs,
		ReduceMs: totalMs - c.mapPhaseMs,
	}
	db.DPrintf(db.ALWAYS, "reduce phase took %vms", pd.ReduceMs)
	if err := c.PutFileJson(mr.MRPhaseStats(c.jobRoot, c.job), 0777, pd); err != nil {
		db.DPrintf(db.ERROR, "PutFileJson %v err %v", mr.MRPhaseStats(c.jobRoot, c.job), err)
	}
}

func (c *Coord) processResult(ch <-chan ftmgr.Tresult[[]byte, []byte], m, r int32) {
	db.DPrintf(db.MR_COORD, "processResults %d %d", m, r)
	nM := int(m)
	nR := int(r)
	nRestart := 0

	// we may recovery with all mappers done and we should kick off
	// the reducers
	if nM >= c.nmaptask {
		c.recordMapPhaseDone()
		if err := c.makeReduceBins(); err != nil {
			db.DFatalf("ReduceBins err %v", err)
		}
	}
	ts := make(map[ftclnt.TaskId]bool)
	for res := range ch {
		db.DPrintf(db.MR_COORD, "processResult: res %v", res)
		if res.Err == nil && res.Status.IsStatusOK() {
			if c.maliciousMapper > 0 && res.Proc.GetProgram() == MALICIOUS_MAPPER_BIN {
				// If running with malicious mapper, then exit status should not be OK.
				// The task should be restarted automatically by the MR FT
				// infrastructure.  If the exit status *was* OK, then the output files
				// won't match, because the malicious mapper doesn't actually do the map
				// (it just touches some buckets it shouldn't have access to). Because of
				// this, letting the coordinator proceed by marking the task as done
				// should cause the test to fail.
				db.DPrintf(db.ERROR, "!!! WARNING: MALICIOUS MAPPER SUCCEEDED !!!")
			}
			r, err := mr.NewResult(res.Status.Data())
			if err != nil {
				db.DFatalf("NewResult %v err %v", res.Status.Data(), err)
			}
			r.MsOuter = res.Ms.Milliseconds()
			db.DPrintf(db.MR_COORD, "Task results %v", r)
			// mark task as done
			start := time.Now()
			encoded, err := ftclnt.Encode(r.OutBin)
			if err != nil {
				db.DFatalf("Encode %v err %v", r.OutBin, err)
			}
			if err := res.Ftclnt.AddTaskOutputs([]ftclnt.TaskId{res.Id}, [][]byte{encoded}, true); err != nil {
				db.DFatalf("MarkDone %v done err %v", res.Id, err)
			}
			db.DPrintf(db.MR_COORD, "MarkDone latency: lat %v", time.Since(start))
			if err := c.AppendFileJson(mr.MRstats(c.jobRoot, c.job), r); err != nil {
				db.DFatalf("Appendfile %v err %v", mr.MRstats(c.jobRoot, c.job), err)
			}
			if r.IsM {
				if _, ok := ts[res.Id]; ok {
					db.DFatalf("task id already finished %v", res.Id)
				}
				ts[res.Id] = true
				nM += 1
				if nM >= c.nmaptask { // kick off reducers?
					c.recordMapPhaseDone()
					if err := c.makeReduceBins(); err != nil {
						db.DFatalf("ReduceBins err %v", err)
					}
				}
			} else {
				nR += 1
				if nR >= c.nreducetask {
					c.recordReducePhaseDone()
					db.DPrintf(db.MR_COORD, "processResult: SubmittedLastTask")
					c.mftclnt.SubmittedLastTask()
					c.rftclnt.SubmittedLastTask()
				}
			}
			db.DPrintf(db.ALWAYS, "tasks done %d/%d\n", nM+nR, c.nmaptask+c.nreducetask)
		} else {
			db.DPrintf(db.MR, "Task failed %v status %v", res.Id, res.Status)
			if res.Status != nil && res.Status.Msg() == mr.RESTART {
				// reducer indicates to run some mappers again
				s := newStringSlice(res.Status.Data().([]interface{}))
				c.restart(s, res.Id)
				nRestart += 1
			} else { // if failure but not restart, rerun task immediately again
				if err := res.Ftclnt.MoveTasks([]ftclnt.TaskId{res.Id}, ftclnt.TODO); err != nil {
					db.DFatalf("MarkRunnable %v err %v", res.Id, err)
				}
			}
			c.stat.Nfail.Add(1)
		}
		if nR+nRestart >= c.nreducetask { // Run mappers again for some errored reducers?
			nM = 0
			nRestart = 0
			ts = make(map[ftclnt.TaskId]bool)
			c.doRestart()
		}
	}
}
