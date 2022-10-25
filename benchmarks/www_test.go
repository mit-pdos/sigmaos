package benchmarks_test

import (
	"path"
	"time"

	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/stat/distuv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/test"
	"sigmaos/www"
)

type WwwJobInstance struct {
	wwwncore proc.Tcore // Number of exclusive cores allocated to each wwwd
	job      string
	ntrials  int
	nclnt    int
	nreq     int
	delay    time.Duration
	poisson  *distuv.Poisson
	ready    chan bool
	sem      *semclnt.SemClnt
	sempath  string
	cpids    []proc.Tpid
	pid      proc.Tpid
	*test.Tstate
}

func MakeWwwJob(ts *test.Tstate, wwwncore proc.Tcore, reqtype string, ntrials, nclnt, nreq int, delay time.Duration) *WwwJobInstance {
	ji := &WwwJobInstance{}
	ji.job = rand.String(16)
	ji.ntrials = ntrials
	ji.nclnt = nclnt
	ji.poisson = &distuv.Poisson{Lambda: 1.0}
	ji.nreq = nreq
	ji.delay = delay
	ji.ready = make(chan bool)
	ji.Tstate = ts

	www.InitWwwFs(ts.FsLib, ji.job)

	ji.sempath = path.Join(www.JobDir(ji.job), "kvclerk-sem")
	ji.sem = semclnt.MakeSemClnt(ts.FsLib, ji.sempath)
	err := ji.sem.Init(0)
	assert.Nil(ji.T, err, "Sem init: %v", err)
	assert.Equal(ji.T, reqtype, "compute")
	return ji
}

func (ji *WwwJobInstance) RunClient(ch chan time.Duration) {
	clnt := www.MakeWWWClnt(ji.FsLib, ji.job)
	var latency time.Duration
	for i := 0; i < ji.nreq; i++ {
		time.Sleep(ji.delay * ji.poisson.Rand())
		start := time.Now()
		err := clnt.MatMul(MAT_SIZE)
		assert.Equal(ji.T, nil, err)
		latency += time.Since(start)
	}
	ch <- latency
}

func (ji *WwwJobInstance) StartWwwJob() {
	a := proc.MakeProc("user/wwwd", []string{ji.job, ""})
	err := ji.Spawn(a)
	assert.Nil(ji.T, err, "Spawn")
	err = ji.WaitStart(a.Pid)
	ji.pid = a.Pid
	assert.Equal(ji.T, nil, err)
	db.DPrintf(db.ALWAYS, "StartWwwJob ntrial %v nclnt %v nreq/clnt %v avgdelay %v", ji.ntrials, ji.nclnt, ji.nreq, ji.delay)
	for i := 1; i <= ji.nclnt; i++ {
		for j := 0; j < ji.ntrials; j++ {
			ch := make(chan time.Duration)
			for c := 0; c < i; c++ {
				go ji.RunClient(ch)
			}
			var totalLatency time.Duration
			for c := 0; c < i; c++ {
				totalLatency += <-ch
			}
			d := totalLatency.Milliseconds()
			db.DPrintf(db.ALWAYS, "trial %v nclnt %d avg latency %vms", j, i, float64(d)/(float64(ji.nreq)*float64(i)))
		}
	}
}

func (ji *WwwJobInstance) Wait() {
	clnt := www.MakeWWWClnt(ji.FsLib, ji.job)
	err := clnt.StopServer(ji.ProcClnt, ji.pid)
	assert.Nil(ji.T, err)
}
