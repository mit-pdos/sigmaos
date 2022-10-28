package www_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/test"
	"sigmaos/www"
)

type Tstate struct {
	*test.Tstate
	*www.WWWClnt
	pid proc.Tpid
	job string
}

func spawn(t *testing.T, ts *Tstate) proc.Tpid {
	a := proc.MakeProc("user/wwwd", []string{ts.job, ""})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return a.Pid
}

func makeTstate(t *testing.T) *Tstate {
	var err error
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)

	ts.job = rd.String(4)

	www.InitWwwFs(ts.Tstate.FsLib, ts.job)

	ts.pid = spawn(t, ts)

	err = ts.WaitStart(ts.pid)
	assert.Nil(t, err)

	ts.WWWClnt = www.MakeWWWClnt(ts.Tstate.FsLib, ts.job)

	return ts
}

func (ts *Tstate) waitWww() {
	err := ts.StopServer(ts.ProcClnt, ts.pid)
	assert.Nil(ts.T, err)
	ts.Shutdown()
}

func TestSandbox(t *testing.T) {
	ts := makeTstate(t)
	ts.waitWww()
}

func TestStatic(t *testing.T) {
	ts := makeTstate(t)

	out, err := ts.GetStatic("hello.html")
	assert.Nil(t, err)
	assert.Contains(t, string(out), "hello")

	out, err = ts.GetStatic("nonexist.html")
	assert.NotNil(t, err, "Out: %v", string(out)) // wget return error because of HTTP not found

	ts.waitWww()
}

func TestView(t *testing.T) {
	ts := makeTstate(t)

	out, err := ts.View()
	assert.Nil(t, err)
	assert.Contains(t, string(out), "Homer")

	ts.waitWww()
}

func TestEdit(t *testing.T) {
	ts := makeTstate(t)

	out, err := ts.Edit("Odyssey")
	assert.Nil(t, err)
	assert.Contains(t, string(out), "Odyssey")

	ts.waitWww()
}

func TestSave(t *testing.T) {
	ts := makeTstate(t)

	out, err := ts.Save()
	assert.Nil(t, err)
	assert.Contains(t, string(out), "Homer")

	ts.waitWww()
}

func matmulClnt(ts *Tstate, matsize, clntid, nreq int, avgslp time.Duration, done chan bool) {
	clnt := www.MakeWWWClnt(ts.FsLib, ts.job)
	for i := 0; i < nreq; i++ {
		slp := avgslp * time.Duration(rd.Uint64()%100) / 100
		db.DPrintf("TEST", "[%v] iteration %v Random sleep %v", clntid, i, slp)
		time.Sleep(slp)
		err := clnt.MatMul(matsize)
		assert.Nil(ts.T, err, "Error matmul: %v", err)
	}
	db.DPrintf("TEST", "[%v] done", clntid)
	done <- true
}

func TestMatMul(t *testing.T) {
	ts := makeTstate(t)

	done := make(chan bool)
	go matmulClnt(ts, 2000, 0, 1, 0, done)
	<-done

	ts.waitWww()
}

func TestMatMulConcurrent(t *testing.T) {
	ts := makeTstate(t)

	N_CLNT := 40
	done := make(chan bool)
	for i := 0; i < N_CLNT; i++ {
		go matmulClnt(ts, 2000, i, 10, 500*time.Millisecond, done)
	}
	for i := 0; i < N_CLNT; i++ {
		<-done
	}

	ts.waitWww()
}
