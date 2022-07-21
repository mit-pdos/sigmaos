package benchmarks_test

import (
	"path"

	"github.com/stretchr/testify/assert"

	"ulambda/groupmgr"
	"ulambda/mr"
	"ulambda/test"
)

type MRJobInstance struct {
	app     string
	jobname string
}

func StartMRJob(ts *test.Tstate, app, jobname string) *groupmgr.GroupMgr {
	job := mr.ReadJobConfig(path.Join("..", "mr", app))
	mr.InitCoordFS(ts.FsLib, jobname, job.Nreduce)
	nmap, err := mr.PrepareJob(ts.FsLib, jobname, job)
	assert.Nil(ts.T, err, "Error PrepareJob: %v", err)
	assert.NotEqual(ts.T, 0, nmap, "Error PrepareJob nmap 0")
	return mr.StartMRJob(ts.FsLib, ts.ProcClnt, jobname, job, mr.NCOORD, nmap, 0, 0)
}
