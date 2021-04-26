package test2pc

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/twopc"
)

type Tstate struct {
	t         *testing.T
	s         *fslib.System
	fsl       *fslib.FsLib
	ch        chan bool
	chPresent chan bool
	mfss      []string
	pid       string
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.ch = make(chan bool)
	ts.chPresent = make(chan bool)

	s, err := fslib.Boot("..")
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	ts.fsl = fslib.MakeFsLib("twopc_test")

	err = ts.fsl.Mkdir(twopc.DIR2PC, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	err = ts.fsl.Mkdir(twopc.TWOPCPREPARED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", twopc.TWOPCPREPARED, err)
	}
	err = ts.fsl.Mkdir(twopc.TWOPCCOMMITTED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", twopc.TWOPCCOMMITTED, err)
	}

	err = ts.fsl.Mkdir(memfsd.MEMFS, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}

	return ts
}

func (ts *Tstate) shutdown() {
	ts.stopMemFSs()
	ts.s.Shutdown(ts.fsl)
}

func (ts *Tstate) spawnMemFS() string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/memfsd"
	a.Args = []string{""}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
	return a.Pid
}

func (ts *Tstate) spawnParticipant(index, opcode string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/test2pc"
	a.Args = []string{index, opcode}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
	return a.Pid
}

func (ts *Tstate) runCoord(t *testing.T, ch chan bool) {
	pid1 := twopc.SpawnCoord(ts.fsl, "restart", nil)
	log.Printf("coord spawned %v\n", pid1)
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, string(ok), "OK")
	ch <- true
}

func (ts *Tstate) startParticipants(n int, opcode string) []string {
	fws := make([]string, 0)
	for r := 0; r < n; r++ {
		var fw string
		if opcode != "" && r == 0 {
			fw = ts.spawnParticipant(strconv.Itoa(r), opcode)
		} else {
			fw = ts.spawnParticipant(strconv.Itoa(r), "")
		}
		fws = append(fws, partname(fw))
	}
	return fws
}

func (ts *Tstate) startMemFSs(n int) []string {
	mfss := make([]string, 0)
	for r := 0; r < n; r++ {
		mfs := ts.spawnMemFS()
		mfss = append(mfss, mfs)
	}
	return mfss
}

func (ts *Tstate) stopMemFSs() {
	for _, mfs := range ts.mfss {
		err := ts.fsl.Remove(memfsd.MEMFS + "/" + mfs + "/")
		assert.Nil(ts.t, err, "Remove")
	}
}

func fn(mfs, f string) string {
	return memfsd.MEMFS + "/" + mfs + "/" + f
}

func (ts *Tstate) setUpParticipants(opcode string) []string {
	const N = 3

	ts.mfss = ts.startMemFSs(N)

	time.Sleep(200 * time.Millisecond)

	err := ts.fsl.MakeFile(fn(ts.mfss[0], "x"), 0777, []byte("x"))
	assert.Nil(ts.t, err, "MakeFile")
	err = ts.fsl.MakeFile(fn(ts.mfss[1], "y"), 0777, []byte("y"))
	assert.Nil(ts.t, err, "MakeFile")

	ti := Tinput{}
	ti.Fns = []string{
		fn(ts.mfss[0], ""),
		fn(ts.mfss[1], ""),
		fn(ts.mfss[2], ""),
	}

	err = ts.fsl.MakeFileJson(memfsd.MEMFS+"/txni", 0777, ti)
	assert.Nil(ts.t, err, "MakeFile")

	fws := ts.startParticipants(N-1, opcode)
	return fws
}

func (ts *Tstate) checkCoord(fws []string, opcode string) {
	pid := twopc.SpawnCoord(ts.fsl, opcode, fws)
	ok, err := ts.fsl.Wait(pid)
	assert.Nil(ts.t, err, "Wait")
	if !strings.HasPrefix(opcode, "crash") {
		assert.Equal(ts.t, "OK", string(ok))
	}
}

func (ts *Tstate) testAbort() {
	b, err := ts.fsl.ReadFile(fn(ts.mfss[0], "x"))
	assert.Nil(ts.t, err, "ReadFile")
	assert.Equal(ts.t, b, []byte("x"))

	b, err = ts.fsl.ReadFile(fn(ts.mfss[2], "y"))
	assert.NotEqual(ts.t, nil, "ReadFile")
}

func (ts *Tstate) testCommit() {
	b, err := ts.fsl.ReadFile(fn(ts.mfss[0], "x"))
	assert.NotEqual(ts.t, nil, "ReadFile")

	b, err = ts.fsl.ReadFile(fn(ts.mfss[2], "y"))
	assert.Nil(ts.t, err, "ReadFile")
	assert.Equal(ts.t, b, []byte("y"))
}

func TestCommit(t *testing.T) {
	ts := makeTstate(t)
	fws := ts.setUpParticipants("")

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "start")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit()

	ts.shutdown()
}

func TestAbort(t *testing.T) {
	ts := makeTstate(t)
	fws := ts.setUpParticipants("crash1")

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "start")

	time.Sleep(100 * time.Millisecond)

	ts.testAbort()

	ts.shutdown()
}

func TestCrash2(t *testing.T) {
	ts := makeTstate(t)
	fws := ts.setUpParticipants("")

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "crash2")

	time.Sleep(100 * time.Millisecond)

	ts.testAbort()

	ts.shutdown()
}

func TestCrash3(t *testing.T) {
	ts := makeTstate(t)
	fws := ts.setUpParticipants("")

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "crash3")

	time.Sleep(100 * time.Millisecond)

	ts.testAbort()

	ts.shutdown()
}
