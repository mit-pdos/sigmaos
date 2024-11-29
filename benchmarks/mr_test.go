package benchmarks_test

import (
	"path/filepath"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
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
	done    int32
	job     *mr.Job
	cm      *groupmgr.GroupMgr
}

func NewMRJobInstance(ts *test.RealmTstate, p *perf.Perf, app, jobRoot, jobname string, memreq proc.Tmem) *MRJobInstance {
	ji := &MRJobInstance{}
	ji.RealmTstate = ts
	ji.p = p
	ji.ready = make(chan bool)
	ji.app = app
	ji.jobRoot = jobRoot
	ji.jobname = jobname
	ji.memreq = memreq
	return ji
}

func (ji *MRJobInstance) PrepareMRJob() {
	jobf, err := mr.ReadJobConfig(filepath.Join("..", "mr/job-descriptions", ji.app))
	assert.Nil(ji.Ts.T, err, "Error ReadJobConfig: %v", err)
	ji.job = jobf
	db.DPrintf(db.TEST, "MR job description: %v", ji.job)
	db.DPrintf(db.TEST, "Prepare MR FS %v", ji.jobname)
	tasks, err := mr.InitCoordFS(ji.FsLib, ji.jobRoot, ji.jobname, ji.job.Nreduce)
	assert.Nil(ji.Ts.T, err, "Error InitCoordFS: %v", err)
	db.DPrintf(db.TEST, "Done prepare MR FS %v", ji.jobname)
	db.DPrintf(db.TEST, "Prepare MR job %v %v", ji.jobname, ji.job)
	nmap, err := mr.PrepareJob(ji.FsLib, tasks, ji.jobRoot, ji.jobname, ji.job)
	db.DPrintf(db.TEST, "Done prepare MR job %v %v", ji.jobname, ji.job)
	ji.nmap = nmap
	assert.Nil(ji.Ts.T, err, "Error PrepareJob: %v", err)
	assert.NotEqual(ji.Ts.T, 0, nmap, "Error PrepareJob nmap 0")
}

func (ji *MRJobInstance) StartMRJob() {
	db.DPrintf(db.TEST, "Start MR job %v %v", ji.jobname, ji.job)
	ji.cm = mr.StartMRJob(ji.SigmaClnt, ji.jobRoot, ji.jobname, ji.job, ji.nmap, ji.memreq, 0)
}

func (ji *MRJobInstance) Wait() {
	mr.WaitJobDone(ji.FsLib, ji.jobRoot, ji.jobname)
}

func (ji *MRJobInstance) WaitJobExit() {
	ji.cm.WaitGroup()
}

func chooseMRJobRoot(ts *test.RealmTstate) string {
	// Choose a UX to host the job dir
	uxSts, err := ts.GetDir(sp.UX)
	assert.Nil(ts.Ts.T, err, "GetDir: %v", err)
	return filepath.Join(sp.UX, sp.Names(uxSts)[0]) + mr.MR
}
