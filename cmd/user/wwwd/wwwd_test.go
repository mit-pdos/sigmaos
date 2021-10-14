package main

import (
	"log"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procbasev1"
	"ulambda/procinit"
	"ulambda/realm"
)

type Tstate struct {
	proc.ProcClnt
	*fslib.FsLib
	t   *testing.T
	e   *realm.TestEnv
	cfg *realm.RealmConfig
	pid string
}

func spawn(t *testing.T, ts *Tstate, pid string) {
	a := &proc.Proc{pid, "bin/user/wwwd", "",
		[]string{pid},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
}

func makeTstate(t *testing.T) *Tstate {
	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})

	ts := &Tstate{}
	bin := "../../../"
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	db.Name("wwwd_test")
	ts.FsLib = fslib.MakeFsLibAddr("wwwd_test", cfg.NamedAddr)
	// ts.ProcClnt = procinit.MakeProcClnt(ts.FsLib, procinit.GetProcLayersMap())
	ts.ProcClnt = procbasev1.MakeProcBaseClnt(ts.FsLib)
	ts.t = t

	ts.pid = proc.GenPid()
	spawn(t, ts, ts.pid)

	err = ts.WaitStart(ts.pid)
	assert.Equal(t, nil, err)

	return ts
}

func (ts *Tstate) waitWww() {
	_, err := exec.Command("wget", "-qO-", "http://localhost:8080/exit/").Output()
	assert.NotEqual(ts.t, nil, err)

	status, err := ts.WaitExit(ts.pid)
	assert.Nil(ts.t, err, "WaitExit error")
	assert.Equal(ts.t, "OK", status, "Exit status wrong")

	log.Printf("SHUTDOWN\n")

	ts.e.Shutdown()
}

func TestSandbox(t *testing.T) {
	ts := makeTstate(t)

	_, err := exec.Command("wget", "-qO-", "http://localhost:8080/exit/").Output()
	assert.NotEqual(t, nil, err)

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
