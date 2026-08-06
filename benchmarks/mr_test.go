package benchmarks_test

import (
	"path/filepath"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	mrcoord "sigmaos/apps/mr/coord"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	fttask "sigmaos/ft/task"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/memblock"
	"sigmaos/util/perf"
	"sigmaos/util/spstats"
)

type MRJobInstance struct {
	*test.RealmTstate
	p       *perf.Perf
	ready   chan bool
	app     string
	jobRoot string
	jobname string
	nmap    int
	// Memory each mapper and each reducer reserves; see mrcoord.StartMRJob.
	mapperMem  proc.Tmem
	reducerMem proc.Tmem
	job        *mr.Job
	cm         *procgroupmgr.ProcGroupMgr
	mftid      fttask.FtTaskSvcId
	rftid      fttask.FtTaskSvcId
	// How many machines to dedicate to hosting the job's data, and the blocker
	// holding them out of the pool once they have been.
	nDedicatedUx int
	blocker      *memblock.Blocker
}

func NewMRJobInstance(ts *test.RealmTstate, p *perf.Perf, app string, jobCfg *mr.Job, jobRoot, jobname string, mapperMem, reducerMem proc.Tmem, nDedicatedUx int) *MRJobInstance {
	ji := &MRJobInstance{}
	ji.RealmTstate = ts
	ji.p = p
	ji.ready = make(chan bool)
	ji.app = app
	ji.job = jobCfg
	ji.jobRoot = jobRoot
	ji.jobname = jobname
	ji.mapperMem = mapperMem
	ji.reducerMem = reducerMem
	ji.nDedicatedUx = nDedicatedUx
	return ji
}

// DedicateUxNodes takes nDedicatedUx machines out of the pool of nodes that will
// accept procs, so that they serve this job's input and intermediate data without
// running any of its mappers or reducers. Returns the kernel IDs it set aside.
//
// Called before PrepareMRJob, and this order matters twice over: the input has to
// be staged on those machines and the bins listed from one of them, and both the
// coordinator and PrepareJob find the dedicated set by reading the directory the
// blockers register in — so the blockers must be up first. Their realm's fsuxd is
// already running by now, since a realm boots one per node when it is created,
// which is why blocking a node's memory doesn't stop it from serving.
func (ji *MRJobInstance) DedicateUxNodes() []string {
	if ji.nDedicatedUx <= 0 {
		return nil
	}
	ji.blocker = memblock.NewBlocker(ji.SigmaClnt)
	kids, err := ji.blocker.Kernels(ji.nDedicatedUx)
	if !assert.Nil(ji.Ts.T, err, "Error select kernels to dedicate: %v", err) {
		return nil
	}
	db.DPrintf(db.ALWAYS, "Dedicating %v machines to hosting MR input: %v", len(kids), kids)
	// Block against the larger of the two requests: a reducer must be kept off
	// these machines as firmly as a mapper.
	reject := ji.mapperMem
	if ji.reducerMem > reject {
		reject = ji.reducerMem
	}
	err = ji.blocker.BlockKernels(kids, reject)
	assert.Nil(ji.Ts.T, err, "Error dedicate UX machines: %v", err)
	// The set the coordinator will read must be the set we chose: if it isn't,
	// something else has blocked memory too and the job would send its data to a
	// machine we know nothing about.
	blocked, err := memblock.Blocked(ji.FsLib)
	assert.Nil(ji.Ts.T, err, "Error read blocked kernels: %v", err)
	assert.Equal(ji.Ts.T, kids, blocked, "Blocked kernels aren't the ones dedicated")
	return kids
}

// ReleaseUxNodes gives the dedicated machines back to the cluster. Safe to call
// when none were dedicated.
func (ji *MRJobInstance) ReleaseUxNodes() {
	if ji.blocker == nil {
		return
	}
	err := ji.blocker.Evict()
	assert.Nil(ji.Ts.T, err, "Error release dedicated UX machines: %v", err)
	ji.blocker = nil
}

func (ji *MRJobInstance) PrepareMRJob() {
	assert.NotNil(ji.Ts.T, ji.job, "No MR job description supplied for app %v", ji.app)
	db.DPrintf(db.TEST, "MR job description: %v", ji.job)
	// If the job specifies an S3 input source, copy the input to the UX servers
	// mappers will read from before setting up the job (which computes the input
	// bins): every server ordinarily, or only the dedicated machines when there
	// are any — staging it only there is what makes a mapper whose path wasn't
	// rewritten fail loudly instead of quietly reading a local copy.
	if ji.job.S3Input != "" {
		dedicated, err := memblock.Blocked(ji.FsLib)
		assert.Nil(ji.Ts.T, err, "Error read dedicated UX machines: %v", err)
		db.DPrintf(db.TEST, "Copy MR job input from S3 %v to UX %v (srvs %v)", ji.job.S3Input, ji.job.Input, dedicated)
		err = mr.CopyS3InputToUx(ji.FsLib, ji.job, dedicated)
		assert.Nil(ji.Ts.T, err, "Error copy S3 input to UX: %v", err)
		db.DPrintf(db.TEST, "Done copy MR job input from S3 %v to UX %v", ji.job.S3Input, ji.job.Input)
	}
	db.DPrintf(db.TEST, "Prepare MR FS %v", ji.jobname)
	tasks, err := mrcoord.InitCoordFS(ji.SigmaClnt, ji.jobRoot, ji.jobname, ji.job.Nreduce)
	assert.Nil(ji.Ts.T, err, "Error InitCoordFS: %v", err)
	ji.mftid = tasks.Mftsrv.Id
	ji.rftid = tasks.Rftsrv.Id
	db.DPrintf(db.TEST, "Done prepare MR FS %v", ji.jobname)
	db.DPrintf(db.TEST, "Prepare MR job %v %v", ji.jobname, ji.job)
	nmap, err := mrcoord.PrepareJob(ji.FsLib, tasks, ji.jobRoot, ji.jobname, ji.job)
	db.DPrintf(db.TEST, "Done prepare MR job %v %v", ji.jobname, ji.job)
	ji.nmap = nmap
	assert.Nil(ji.Ts.T, err, "Error PrepareJob: %v", err)
	assert.NotEqual(ji.Ts.T, 0, nmap, "Error PrepareJob nmap 0")
	db.DPrintf(db.ALWAYS, "MR job %v expected stats: nmappers %d nreducers %d binsz %d mapperMem %vMB reducerMem %vMB", ji.jobname, nmap, ji.job.Nreduce, ji.job.Binsz, ji.mapperMem, ji.reducerMem)
}

func (ji *MRJobInstance) StartMRJob() {
	db.DPrintf(db.TEST, "Start MR job %v %v", ji.jobname, ji.job)
	ji.cm = mrcoord.StartMRJob(ji.SigmaClnt, ji.jobRoot, ji.jobname, ji.job, ji.nmap, ji.mapperMem, ji.reducerMem, 0, ji.mftid, ji.rftid)
}

func (ji *MRJobInstance) Wait() {
	mr.WaitJobDone(ji.FsLib, ji.jobRoot, ji.jobname)
}

// Report the durations recorded by the coordinator and return its end-to-end
// figure, which is what the benchmark reports as the job's latency: it measures
// task execution from inside the coordinator, so it excludes spawning the
// coordinator, its leader election, and fttask setup — scaffolding the driver's
// own timer cannot separate out. Zero if the coordinator didn't record one (an
// older mr-coord binary), in which case the caller keeps its own measurement.
// Must be called after the job is done (i.e. after Wait).
func (ji *MRJobInstance) PrintPhaseDurations() time.Duration {
	pd, err := mrcoord.ReadPhaseDurations(ji.FsLib, ji.jobRoot, ji.jobname)
	if !assert.Nil(ji.Ts.T, err, "Error read MR phase durations: %v", err) {
		return 0
	}
	db.DPrintf(db.ALWAYS, "MR job %v e2e %vms map phase %vms reduce phase %vms", ji.jobname, pd.E2eMs, pd.MapMs, pd.ReduceMs)
	return time.Duration(pd.E2eMs) * time.Millisecond
}

// WaitJobExit waits for the job's coordinator(s) to exit, and fails the
// benchmark if any mapper or reducer failed along the way, or if the reducers
// didn't read everything the mappers wrote.
//
// A job that loses tasks still finishes and still reports phase durations — the
// MR fault-tolerance machinery just re-runs them — so a benchmark can otherwise
// report a perfectly plausible number for a run that was quietly retrying work.
// The coordinator counts what happened in its AStat and returns it in its exit
// status (see coord.Work), which is where these counters come from.
func (ji *MRJobInstance) WaitJobExit() {
	stati := ji.cm.WaitGroup()
	st := spstats.NewTcounterSnapshot()
	for _, s := range stati {
		if !s.IsStatusOK() {
			assert.True(ji.Ts.T, false, "MR job %v: coordinator exited with status %v", ji.jobname, s)
			continue
		}
		stro, err := spstats.UnmarshalTcounterSnapshot(s.Data())
		if !assert.Nil(ji.Ts.T, err, "Error unmarshal MR coord stats: %v", err) {
			continue
		}
		// A restarted coordinator reports its own counters; keep the ones from
		// the coordinator that ran the tasks.
		if stro.Counters["Nmap"] > 0 || stro.Counters["Nreduce"] > 0 {
			st = stro
		}
	}
	db.DPrintf(db.ALWAYS, "MR job %v coord stats %v", ji.jobname, st)
	// Nfail counts mappers and reducers that exited non-OK; the recover counters
	// count tasks re-run because a reducer couldn't read a mapper's output. Any
	// of them means the run's timings include re-executed work.
	clean := true
	for _, c := range []string{"Nfail", "Nrestart", "NrecoverMap", "NrecoverReduce"} {
		if !assert.Equal(ji.Ts.T, int64(0), st.Counters[c],
			"MR job %v: %v = %v, so some mappers/reducers failed and were re-run; this run's numbers include repeated work (coord stats %v)",
			ji.jobname, c, st.Counters[c], st) {
			clean = false
		}
	}
	// Every mapper shard is read by exactly one reducer, so the bytes the
	// reducers read must equal the bytes the mappers wrote. Checking it catches
	// the failure mode none of the counters above can see: a read path that
	// silently returns short (or nothing) leaves every task exiting OK, the
	// phases timed, and the answer wrong or empty. Only meaningful when no task
	// ran twice, since a re-run mapper's output is counted again.
	if clean {
		assert.Equal(ji.Ts.T, st.Counters["MapOutBytes"], st.Counters["ReduceInBytes"],
			"MR job %v: reducers read %v bytes but mappers wrote %v — the reduce phase did not see all of the intermediate output, so this run's result is wrong (coord stats %v)",
			ji.jobname, st.Counters["ReduceInBytes"], st.Counters["MapOutBytes"], st)
	}
}

func chooseMRJobRoot(ts *test.RealmTstate) string {
	// Choose a UX to host the job dir
	uxSts, err := ts.GetDir(sp.UX)
	assert.Nil(ts.Ts.T, err, "GetDir: %v", err)
	return filepath.Join(sp.UX, sp.Names(uxSts)[0]) + mr.MR
}
