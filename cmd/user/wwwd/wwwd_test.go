package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
	"ulambda/procclnt"
)

type Tstate struct {
	*procclnt.ProcClnt
	*fslib.FsLib

	t   *testing.T
	s   *kernel.System
	pid string
}

func piddir(pid string) string {
	return "pids/" + pid + "/pids/"
}

func childdir(pid string) string {
	return "pids/" + pid + "/pids/" + pid
}

func spawn(t *testing.T, ts *Tstate) string {
	a := proc.MakeProc("bin/user/wwwd", []string{""})
	a.PidDir = piddir(a.Pid)
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return a.Pid
}

func makeTstate(t *testing.T) *Tstate {
	var err error
	ts := &Tstate{}
	ts.t = t
	ts.s, ts.FsLib, err = kernel.MakeSystemAll("wwwd_test", "../../../")
	assert.Nil(t, err, "Start")
	ts.FsLib = fslib.MakeFsLibAddr("wwwd_test", fslib.Named())
	ts.ProcClnt = procclnt.MakeProcClntInit(ts.FsLib, fslib.Named())
	ts.pid = spawn(t, ts)

	err = ts.WaitStart(childdir(ts.pid))
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

	status, err := ts.WaitExit(childdir(ts.pid))
	assert.Nil(ts.t, err, "WaitExit error")
	assert.Equal(ts.t, "OK", status, "Exit status wrong")

	r := <-ch
	assert.NotEqual(ts.t, nil, r)

	ts.s.Shutdown(ts.FsLib)
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
