package benchmarks_test

import (
	"path/filepath"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	mrcoord "sigmaos/apps/mr/coord"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	fttask "sigmaos/ft/task"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
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
	memreq  proc.Tmem
	job     *mr.Job
	cm      *procgroupmgr.ProcGroupMgr
	mftid   fttask.FtTaskSvcId
	rftid   fttask.FtTaskSvcId
}

func NewMRJobInstance(ts *test.RealmTstate, p *perf.Perf, app string, jobCfg *mr.Job, jobRoot, jobname string, memreq proc.Tmem) *MRJobInstance {
	ji := &MRJobInstance{}
	ji.RealmTstate = ts
	ji.p = p
	ji.ready = make(chan bool)
	ji.app = app
	ji.job = jobCfg
	ji.jobRoot = jobRoot
	ji.jobname = jobname
	ji.memreq = memreq
	return ji
}

func (ji *MRJobInstance) PrepareMRJob() {
	assert.NotNil(ji.Ts.T, ji.job, "No MR job description supplied for app %v", ji.app)
	db.DPrintf(db.TEST, "MR job description: %v", ji.job)
	// If the job specifies an S3 input source, copy the input to every UX
	// server before setting up the job (which computes the input bins)
	if ji.job.S3Input != "" {
		db.DPrintf(db.TEST, "Copy MR job input from S3 %v to UX %v", ji.job.S3Input, ji.job.Input)
		err := mr.CopyS3InputToUx(ji.FsLib, ji.job)
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
	db.DPrintf(db.ALWAYS, "MR job %v expected stats: nmappers %d nreducers %d binsz %d memreq %vMB", ji.jobname, nmap, ji.job.Nreduce, ji.job.Binsz, ji.memreq)
}

func (ji *MRJobInstance) StartMRJob() {
	db.DPrintf(db.TEST, "Start MR job %v %v", ji.jobname, ji.job)
	ji.cm = mrcoord.StartMRJob(ji.SigmaClnt, ji.jobRoot, ji.jobname, ji.job, ji.nmap, ji.memreq, 0, ji.mftid, ji.rftid)
}

func (ji *MRJobInstance) Wait() {
	mr.WaitJobDone(ji.FsLib, ji.jobRoot, ji.jobname)
}

// Report the map and reduce phase durations recorded by the coordinator. Must
// be called after the job is done (i.e., after Wait).
func (ji *MRJobInstance) PrintPhaseDurations() {
	pd, err := mrcoord.ReadPhaseDurations(ji.FsLib, ji.jobRoot, ji.jobname)
	assert.Nil(ji.Ts.T, err, "Error read MR phase durations: %v", err)
	db.DPrintf(db.ALWAYS, "MR job %v map phase %vms reduce phase %vms", ji.jobname, pd.MapMs, pd.ReduceMs)
}

// WaitJobExit waits for the job's coordinator(s) to exit, and fails the
// benchmark if any mapper or reducer failed along the way.
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
	for _, c := range []string{"Nfail", "Nrestart", "NrecoverMap", "NrecoverReduce"} {
		assert.Equal(ji.Ts.T, int64(0), st.Counters[c],
			"MR job %v: %v = %v, so some mappers/reducers failed and were re-run; this run's numbers include repeated work (coord stats %v)",
			ji.jobname, c, st.Counters[c], st)
	}
}

func chooseMRJobRoot(ts *test.RealmTstate) string {
	// Choose a UX to host the job dir
	uxSts, err := ts.GetDir(sp.UX)
	assert.Nil(ts.Ts.T, err, "GetDir: %v", err)
	return filepath.Join(sp.UX, sp.Names(uxSts)[0]) + mr.MR
}
