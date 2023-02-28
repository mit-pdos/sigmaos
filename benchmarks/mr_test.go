package benchmarks_test

import (
	"path"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/groupmgr"
	"sigmaos/mr"
	"sigmaos/perf"
	"sigmaos/test"
)

type MRJobInstance struct {
	*test.RealmTstate
	p       *perf.Perf
	ready   chan bool
	app     string
	jobname string
	nmap    int
	done    int32
	job     *mr.Job
	cm      *groupmgr.GroupMgr
}

func MakeMRJobInstance(ts *test.RealmTstate, p *perf.Perf, app, jobname string) *MRJobInstance {
	ji := &MRJobInstance{}
	ji.RealmTstate = ts
	ji.p = p
	ji.ready = make(chan bool)
	ji.app = app
	ji.jobname = jobname
	return ji
}

func (ji *MRJobInstance) PrepareMRJob() {
	ji.job = mr.ReadJobConfig(path.Join("..", "mr", ji.app))
	mr.InitCoordFS(ji.FsLib, ji.jobname, ji.job.Nreduce)
	nmap, err := mr.PrepareJob(ji.FsLib, ji.jobname, ji.job)
	ji.nmap = nmap
	assert.Nil(ji.T, err, "Error PrepareJob: %v", err)
	assert.NotEqual(ji.T, 0, nmap, "Error PrepareJob nmap 0")
}

func (ji *MRJobInstance) StartMRJob() {
	ji.cm = mr.StartMRJob(ji.FsLib, ji.ProcClnt, ji.jobname, ji.job, mr.NCOORD, ji.nmap, 0, 0)
}

func (ji *MRJobInstance) Wait() {
	ji.cm.Wait()
}
