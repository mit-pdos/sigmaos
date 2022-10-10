package www_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestMatMul(t *testing.T) {
	ts := makeTstate(t)

	err := ts.MatMul(2000)
	assert.Nil(t, err)
	ts.waitWww()
}
