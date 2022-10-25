package benchmarks_test

import (
	"path"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/test"
	"sigmaos/www"
)

const (
	K8S_PORT = ":32585"
)

type WwwJobInstance struct {
	sigmaos    bool
	k8ssrvaddr string
	wwwncore   proc.Tcore // Number of exclusive cores allocated to each wwwd
	job        string
	ntrials    int
	nclnt      int
	nreq       int
	delay      time.Duration
	ready      chan bool
	sem        *semclnt.SemClnt
	sempath    string
	cpids      []proc.Tpid
	pid        proc.Tpid
	*test.Tstate
}

func MakeWwwJob(ts *test.Tstate, sigmaos bool, wwwncore proc.Tcore, reqtype string, ntrials, nclnt, nreq int, delay time.Duration) *WwwJobInstance {
	ji := &WwwJobInstance{}
	ji.sigmaos = sigmaos
	ji.job = rand.String(16)
	ji.ntrials = ntrials
	ji.nclnt = nclnt
	ji.nreq = nreq
	ji.delay = delay
	ji.ready = make(chan bool)
	ji.Tstate = ts

	www.InitWwwFs(ts.FsLib, ji.job)

	if !sigmaos {
		ip, err := fidclnt.LocalIP()
		assert.Nil(ji.T, err, "Error LocalIP: %v", err)
		ji.k8ssrvaddr = ip + K8S_PORT
	}

	ji.sempath = path.Join(www.JobDir(ji.job), "kvclerk-sem")
	ji.sem = semclnt.MakeSemClnt(ts.FsLib, ji.sempath)
	err := ji.sem.Init(0)
	assert.Nil(ji.T, err, "Sem init: %v", err)
	assert.Equal(ji.T, reqtype, "compute")
	return ji
}

func (ji *WwwJobInstance) RunClient(i int, ch chan time.Duration) {
	var clnt *www.WWWClnt
	if ji.sigmaos {
		clnt = www.MakeWWWClnt(ji.FsLib, ji.job)
	} else {
		clnt = www.MakeWWWClntAddr([]string{ji.k8ssrvaddr})
	}
	var latency time.Duration
	for i := 0; i < ji.nreq; i++ {
		slp := ji.delay * time.Duration(rand.Uint64()%100) / 100
		db.DPrintf("WWWD_TEST", "[%v] Random sleep %v", i, slp)
		time.Sleep(slp)
		start := time.Now()
		err := clnt.MatMul(MAT_SIZE)
		assert.Equal(ji.T, nil, err)
		latency += time.Since(start)
	}
	db.DPrintf("WWWD_TEST", "[%v] done", i)
	ch <- latency
}

func (ji *WwwJobInstance) StartWwwJob() {
	if ji.sigmaos {
		a := proc.MakeProc("user/wwwd", []string{ji.job, ""})
		err := ji.Spawn(a)
		assert.Nil(ji.T, err, "Spawn")
		err = ji.WaitStart(a.Pid)
		ji.pid = a.Pid
		assert.Equal(ji.T, nil, err)
	}
	db.DPrintf(db.ALWAYS, "StartWwwJob ntrial %v nclnt %v nreq/clnt %v avgdelay %v", ji.ntrials, ji.nclnt, ji.nreq, ji.delay)
	for i := 1; i <= ji.nclnt; i++ {
		for j := 0; j < ji.ntrials; j++ {
			ch := make(chan time.Duration)
			for c := 0; c < i; c++ {
				db.DPrintf("WWWD_TEST", "Start client %v", i)
				go ji.RunClient(i, ch)
			}
			var totalLatency time.Duration
			for c := 0; c < i; c++ {
				totalLatency += <-ch
				db.DPrintf("WWWD_TEST", "Done client %v", i)
			}
			d := totalLatency.Milliseconds()
			db.DPrintf(db.ALWAYS, "trial %v nclnt %d avg latency %vms", j, i, float64(d)/(float64(ji.nreq)*float64(i)))
		}
	}
}

func (ji *WwwJobInstance) Wait() {
	if ji.sigmaos {
		clnt := www.MakeWWWClnt(ji.FsLib, ji.job)
		err := clnt.StopServer(ji.ProcClnt, ji.pid)
		assert.Nil(ji.T, err)
	}
}
