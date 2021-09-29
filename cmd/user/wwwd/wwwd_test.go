package main

import (
	"io"
	"log"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/realm"
)

type Tstate struct {
	*fslib.FsLib
	t   *testing.T
	e   *realm.TestEnv
	cfg *realm.RealmConfig
	s   *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	bin := "../../../"
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg
	ts.s = kernel.MakeSystemNamedAddr(bin, cfg.NamedAddr)

	db.Name("fslib_test")
	ts.FsLib = fslib.MakeFsLibAddr("fslibtest", cfg.NamedAddr)
	ts.t = t

	return ts
}

func TestStatic(t *testing.T) {
	ts := makeTstate(t)

	cmd := exec.Command("../../../bin/user/wwwd")
	stderr, err := cmd.StderrPipe()
	assert.Equal(t, nil, err)

	err = cmd.Start()
	assert.Equal(t, nil, err)

	time.Sleep(100 * time.Millisecond)

	out, err := exec.Command("wget", "-qO-", "http://localhost:8080/static/hello.html").Output()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "hello")

	out, err = exec.Command("wget", "-qO-", "http://localhost:8080/static/nonexist.html").Output()
	log.Printf("error: %v %v\n", err, string(out))
	assert.Equal(t, nil, err)
	log.Printf("error: %v\n", string(out))

	err = cmd.Process.Kill()
	assert.Equal(t, nil, err)

	s, _ := io.ReadAll(stderr)
	log.Printf("wwwd: stderr %s", s)

	ts.s.Shutdown()
	ts.e.Shutdown()
}

func TestView(t *testing.T) {
	ts := makeTstate(t)

	cmd := exec.Command("../../../bin/user/wwwd")
	stderr, err := cmd.StderrPipe()
	assert.Equal(t, nil, err)

	err = cmd.Start()
	assert.Equal(t, nil, err)

	time.Sleep(100 * time.Millisecond)

	out, err := exec.Command("wget", "-qO-", "http://localhost:8080/view/books").Output()
	assert.Equal(t, nil, err)
	assert.Contains(t, string(out), "Homer")

	err = cmd.Process.Kill()
	assert.Equal(t, nil, err)

	s, _ := io.ReadAll(stderr)
	log.Printf("wwwd: stderr %s", s)

	ts.s.Shutdown()
	ts.e.Shutdown()
}
