package benchmarks_test

import (
	"path"

	"github.com/stretchr/testify/assert"

	"sigmaos/groupmgr"
	"sigmaos/mr"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/test"
)

type MRJobInstance struct {
	*test.RealmTstate
	p       *perf.Perf
	ready   chan bool
	app     string
	jobname string
	nmap    int
	memreq  proc.Tmem
	done    int32
	job     *mr.Job
	cm      *groupmgr.GroupMgr
}

func NewMRJobInstance(ts *test.RealmTstate, p *perf.Perf, app, jobname string, memreq proc.Tmem) *MRJobInstance {
	ji := &MRJobInstance{}
	ji.RealmTstate = ts
	ji.p = p
	ji.ready = make(chan bool)
	ji.app = app
	ji.jobname = jobname
	ji.memreq = memreq
	return ji
}

func (ji *MRJobInstance) PrepareMRJob() {
	ji.job = mr.ReadJobConfig(path.Join("..", "mr", ji.app))
	mr.InitCoordFS(ji.FsLib, ji.jobname, ji.job.Nreduce)
	nmap, err := mr.PrepareJob(ji.FsLib, ji.jobname, ji.job)
	ji.nmap = nmap
	assert.Nil(ji.Ts.T, err, "Error PrepareJob: %v", err)
	assert.NotEqual(ji.Ts.T, 0, nmap, "Error PrepareJob nmap 0")
}

func (ji *MRJobInstance) StartMRJob() {
	ji.cm = mr.StartMRJob(ji.SigmaClnt, ji.jobname, ji.job, mr.NCOORD, ji.nmap, 0, 0, ji.memreq)
}

func (ji *MRJobInstance) Wait() {
	mr.WaitJobDone(ji.FsLib, ji.jobname)
}

func (ji *MRJobInstance) WaitJobExit() {
	ji.cm.WaitGroup()
}
