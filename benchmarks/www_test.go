package benchmarks_test

import (
	"path"
	"time"

	"github.com/stretchr/testify/assert"

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
	ready    chan bool
	sem      *semclnt.SemClnt
	sempath  string
	cpids    []proc.Tpid
	pid      proc.Tpid
	*test.Tstate
}

func MakeWwwJob(ts *test.Tstate, wwwncore proc.Tcore, reqtype string) *WwwJobInstance {
	ji := &WwwJobInstance{}
	ji.job = rand.String(16)
	ji.ready = make(chan bool)
	ji.Tstate = ts

	www.InitWwwFs(ts.FsLib, ji.job)

	ji.sempath = path.Join(www.JobDir(ji.job), "kvclerk-sem")
	ji.sem = semclnt.MakeSemClnt(ts.FsLib, ji.sempath)
	err := ji.sem.Init(0)
	assert.Nil(ji.T, err, "Sem init: %v", err)
	return ji
}

func (ji *WwwJobInstance) RunClient(ch chan bool) {
	clnt := www.MakeWWWClnt(ji.FsLib, ji.job)
	err := clnt.MatMul(MAT_SIZE)
	assert.Equal(ji.T, nil, err)
	ch <- true
}

func (ji *WwwJobInstance) StartWwwJob() {
	a := proc.MakeProc("user/wwwd", []string{ji.job, ""})
	err := ji.Spawn(a)
	assert.Nil(ji.T, err, "Spawn")
	err = ji.WaitStart(a.Pid)
	ji.pid = a.Pid
	assert.Equal(ji.T, nil, err)
	for i := 1; i <= N_CLNT; i++ {
		ch := make(chan bool)
		start := time.Now()
		for c := 0; c < i; c++ {
			go ji.RunClient(ch)
		}
		for c := 0; c < i; c++ {
			<-ch
		}
		d := time.Since(start).Milliseconds()
		db.DPrintf(db.ALWAYS, "nclnt %d take %v(ms)\n", i, d)
	}
}

func (ji *WwwJobInstance) Wait() {
	clnt := www.MakeWWWClnt(ji.FsLib, ji.job)
	err := clnt.StopServer(ji.ProcClnt, ji.pid)
	assert.Nil(ji.T, err)
}
