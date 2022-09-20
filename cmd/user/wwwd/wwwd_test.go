package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/proc"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	pid proc.Tpid
}

func spawn(t *testing.T, ts *Tstate) proc.Tpid {
	a := proc.MakeProc("user/wwwd", []string{""})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return a.Pid
}

func makeTstate(t *testing.T) *Tstate {
	var err error
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.pid = spawn(t, ts)

	err = ts.WaitStart(ts.pid)
	assert.Equal(t, nil, err)

	// ts.Exited(proc.GetPid(), "OK")

	return ts
}

func (ts *Tstate) waitWww() {
	ch := make(chan error)
	go func() {
		_, err := exec.Command("wget", "-qO-", "http://localhost:8080/exit/").Output()
		ch <- err
	}()

	status, err := ts.WaitExit(ts.pid)
	assert.Nil(ts.T, err, "WaitExit error")
	assert.True(ts.T, status.IsStatusEvicted(), "Exit status wrong")

	r := <-ch
	assert.NotEqual(ts.T, nil, r)

	ts.Shutdown()
}

func TestSandbox(t *testing.T) {
	ts := makeTstate(t)
	ts.waitWww()
}

func TestStatic(t *testing.T) {
	ts := makeTstate(t)

	out, err := exec.Command("wget", "-qO-", "http://localhost:8080/static/hello.html").Output()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "hello")

	out, err = exec.Command("wget", "-qO-", "http://localhost:8080/static/nonexist.html").Output()
	assert.NotEqual(t, nil, err) // wget return error because of HTTP not found

	ts.waitWww()
}

func TestView(t *testing.T) {
	ts := makeTstate(t)

	out, err := exec.Command("wget", "-qO-", "http://localhost:8080/book/view/").Output()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "Homer")

	ts.waitWww()
}

func TestEdit(t *testing.T) {
	ts := makeTstate(t)

	out, err := exec.Command("wget", "-qO-", "http://localhost:8080/book/edit/Odyssey").Output()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "Odyssey")

	ts.waitWww()
}

func TestSave(t *testing.T) {
	ts := makeTstate(t)

	out, err := exec.Command("wget", "-qO-", "--post-data", "title=Odyssey", "http://localhost:8080/book/save/Odyssey").Output()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "Homer")

	ts.waitWww()
}
