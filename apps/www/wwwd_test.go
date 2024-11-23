package www_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/www"
	db "sigmaos/debug"
	"sigmaos/proc"
	rd "sigmaos/util/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	*www.WWWClnt
	pid sp.Tpid
	job string
}

func spawn(t *testing.T, ts *Tstate) sp.Tpid {
	a := proc.NewProc("wwwd", []string{ts.job, ""})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return a.GetPid()
}

func newTstate(t1 *test.Tstate) *Tstate {
	var err error
	ts := &Tstate{}
	ts.Tstate = t1

	ts.Tstate.MkDir(www.TMP, 0777)

	ts.job = rd.String(4)

	err = www.InitWwwFs(ts.Tstate.FsLib, ts.job)
	assert.Nil(t1.T, err)

	ts.pid = spawn(t1.T, ts)

	err = ts.WaitStart(ts.pid)
	assert.Nil(t1.T, err)

	clnt, err := www.NewWWWClnt(ts.Tstate.FsLib, ts.job)
	assert.Nil(t1.T, err)
	ts.WWWClnt = clnt

	return ts
}

func (ts *Tstate) waitWww() {
	err := ts.StopServer(ts.ProcAPI, ts.pid)
	assert.Nil(ts.T, err)
	ts.Shutdown()
}

func TestCompile(t *testing.T) {
}

func TestSandbox(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1)
	ts.waitWww()
}

func TestStatic(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1)

	out, err := ts.GetStatic("hello.html")
	assert.Nil(t, err)
	assert.Contains(t, string(out), "hello")

	out, err = ts.GetStatic("nonexist.html")
	assert.NotNil(t, err, "Out: %v", string(out)) // wget return error because of HTTP not found

	ts.waitWww()
}

func matmulClnt(ts *Tstate, matsize, clntid, nreq int, avgslp time.Duration, done chan bool) {
	clnt, err := www.NewWWWClnt(ts.Tstate.FsLib, ts.job)
	assert.Nil(ts.T, err, "Err WWWClnt: %v", err)
	for i := 0; i < nreq; i++ {
		slp := avgslp * time.Duration(rd.Uint64()%100) / 100
		db.DPrintf(db.TEST, "[%v] iteration %v Random sleep %v", clntid, i, slp)
		time.Sleep(slp)
		err := clnt.MatMul(matsize)
		assert.Nil(ts.T, err, "Error matmul: %v", err)
	}
	db.DPrintf(db.TEST, "[%v] done", clntid)
	done <- true
}

const N = 100

func TestMatMul(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1)

	done := make(chan bool)
	go matmulClnt(ts, N, 0, 1, 0, done)
	<-done

	ts.waitWww()
}

func TestMatMulConcurrent(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1)

	N_CLNT := 5
	done := make(chan bool)
	for i := 0; i < N_CLNT; i++ {
		go matmulClnt(ts, N, i, 10, 500*time.Millisecond, done)
	}
	for i := 0; i < N_CLNT; i++ {
		<-done
	}

	ts.waitWww()
}
