package www_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/test"
	"sigmaos/www"
	"sigmaos/wwwclnt"
)

type Tstate struct {
	*test.Tstate
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

	www.InitWwwFs(ts.FsLib, ts.job)

	ts.pid = spawn(t, ts)

	err = ts.WaitStart(ts.pid)
	assert.Equal(t, nil, err)

	// ts.Exited(proc.GetPid(), "OK")

	return ts
}

func (ts *Tstate) waitWww() {
	err := www.StopServer(ts.ProcClnt, ts.pid)
	assert.Nil(ts.T, err)
	ts.Shutdown()
}

func TestSandbox(t *testing.T) {
	ts := makeTstate(t)
	ts.waitWww()
}

func TestStatic(t *testing.T) {
	ts := makeTstate(t)

	out, err := wwwclnt.Get("hello.html")
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "hello")

	out, err = wwwclnt.Get("nonexist.html")
	assert.NotEqual(t, nil, err) // wget return error because of HTTP not found

	ts.waitWww()
}

func TestView(t *testing.T) {
	ts := makeTstate(t)

	out, err := wwwclnt.View()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "Homer")

	ts.waitWww()
}

func TestEdit(t *testing.T) {
	ts := makeTstate(t)

	out, err := wwwclnt.Edit("Odyssey")
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "Odyssey")

	ts.waitWww()
}

func TestSave(t *testing.T) {
	ts := makeTstate(t)

	out, err := wwwclnt.Save()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "Homer")

	ts.waitWww()
}

func TestMatMul(t *testing.T) {
	ts := makeTstate(t)

	err := wwwclnt.MatMul(2000)
	assert.Equal(t, nil, err)
	ts.waitWww()
}
