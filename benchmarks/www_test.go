package benchmarks_test

import (
	"log"
	"os/exec"
	"path"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/test"
	"sigmaos/www"
	"sigmaos/wwwclnt"
)

type WwwJobInstance struct {
	nwwwd     int        // Number of wwwds to run test with
	nclnts    []int      // Number of clients
	wwwncore  proc.Tcore // Number of exclusive cores allocated to each wwwd
	clntncore proc.Tcore // Number of exclusive cores allocated to each client
	job       string
	ready     chan bool
	sem       *semclnt.SemClnt
	sempath   string
	cpids     []proc.Tpid
	pid       proc.Tpid
	*test.Tstate
}

func MakeWwwJob(ts *test.Tstate, nwwwd int, nclnts []int, wwwncore, clntncore proc.Tcore) *WwwJobInstance {
	ji := &WwwJobInstance{}
	ji.nwwwd = nwwwd
	ji.nclnts = nclnts
	ji.clntncore = clntncore
	ji.job = rand.String(16)
	ji.ready = make(chan bool)
	ji.Tstate = ts

	www.InitWwwFs(ts.FsLib, ji.job)

	ji.sempath = path.Join(www.JobDir(ji.job), "kvclerk-sem")
	ji.sem = semclnt.MakeSemClnt(ts.FsLib, ji.sempath)
	err := ji.sem.Init(0)
	assert.Nil(ji.T, err, "Sem init: %v", err)
	maxNclnts := 0
	for _, nc := range nclnts {
		if maxNclnts < nc {
			maxNclnts = nc
		}
	}
	return ji
}

func (ji *WwwJobInstance) RunClient(ch chan bool) {
	err := wwwclnt.MatMul(4000)
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
	for i := 1; i < 4; i++ {
		ch := make(chan bool)
		start := time.Now()
		for c := 0; c < i; c++ {
			go ji.RunClient(ch)
		}
		for c := 0; c < i; c++ {
			<-ch
		}
		d := time.Since(start).Milliseconds()
		log.Printf("nclnt %d take %v(ms)\n", i, d)
	}
}

func (ji *WwwJobInstance) Wait() {
	// wait until test is done
	ch := make(chan error)
	go func() {
		_, err := exec.Command("wget", "-qO-", "http://localhost:8080/exit/").Output()
		ch <- err
	}()

	status, err := ji.WaitExit(ji.pid)
	assert.Nil(ji.T, err, "WaitExit error")
	assert.True(ji.T, status.IsStatusEvicted(), "Exit status wrong")

	r := <-ch
	assert.NotEqual(ji.T, nil, r)
}
