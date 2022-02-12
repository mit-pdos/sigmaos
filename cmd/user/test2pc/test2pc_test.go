package main

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/groupmgr"
	"ulambda/kernel"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/twopc"
)

type Tstate struct {
	*kernel.System
	t         *testing.T
	ch        chan bool
	chPresent chan bool
	mfss      []string
	pid       string
}

const (
	NCOORD = 3
)

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.ch = make(chan bool)
	ts.chPresent = make(chan bool)
	ts.System = kernel.MakeSystemAll("fsux_test", "../../../")

	err := ts.Mkdir(twopc.DIR2PC, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	err = ts.Mkdir(twopc.TWOPCPREPARED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", twopc.TWOPCPREPARED, err)
	}
	err = ts.Mkdir(twopc.TWOPCCOMMITTED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", twopc.TWOPCCOMMITTED, err)
	}

	err = ts.Mkdir(np.MEMFS, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}

	return ts
}

func (ts *Tstate) shutdown(fws []string) {
	for _, pid := range fws {
		log.Printf("collect %v\n", pid)
		ts.WaitExit(pid)
	}
	ts.stopMemFSs()
	ts.Shutdown()
}

func (ts *Tstate) cleanup() {
	err := ts.Remove(np.MEMFS + "/txni")
	assert.Nil(ts.t, err, "Remove txni")
}

func (ts *Tstate) spawnMemFS() string {
	p := proc.MakeProc("bin/user/memfsd", []string{""})
	ts.Spawn(p)
	ts.WaitStart(p.Pid)
	return p.Pid
}

func (ts *Tstate) spawnParticipant(index, opcode string) string {
	p := proc.MakeProc("bin/user/test2pc", []string{index, opcode})
	ts.Spawn(p)
	return p.Pid
}

func (ts *Tstate) spawnCoord(opcode string, fws []string) string {
	p := proc.MakeProc("bin/user/twopc-coord", append([]string{opcode}, fws...))
	ts.Spawn(p)
	// log.Printf("coord spawned %v\n", p.Pid)
	return p.Pid
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
		fws = append(fws, fw)
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
		err := ts.Evict(mfs)
		assert.Nil(ts.t, err, "Remove")
		ts.WaitExit(mfs)
	}
}

func fn(mfs, f string) string {
	return np.MEMFS + "/" + mfs + "/" + f
}

func (ts *Tstate) setUpMemFSs(N int) {
	ts.mfss = ts.startMemFSs(N)
}

func (ts *Tstate) setUpFiles(mfs0 string, mfs1 string) {
	err := ts.MakeFile(fn(mfs0, "x"), 0777, np.OWRITE, []byte("x"))
	assert.Nil(ts.t, err, "MakeFile")
	err = ts.MakeFile(fn(mfs1, "y"), 0777, np.OWRITE, []byte("y"))
	assert.Nil(ts.t, err, "MakeFile")
}

func (ts *Tstate) setUpParticipants(opcode string, N int) []string {
	ti := Tinput{}
	ti.Fns = []string{
		fn(ts.mfss[0], ""),
		fn(ts.mfss[1], ""),
		fn(ts.mfss[2], ""),
	}

	err := ts.MakeFileJson(np.MEMFS+"/txni", 0777, ti)
	assert.Nil(ts.t, err, "MakeFile")

	fws := ts.startParticipants(N, opcode)
	return fws
}

func (ts *Tstate) checkCoord(fws []string, opcode string) {
	pid := ts.spawnCoord(opcode, fws)
	status, err := ts.WaitExit(pid)
	assert.Nil(ts.t, err, "WaitStart")
	if !strings.HasPrefix(opcode, "crash") {
		assert.True(ts.t, status.IsStatusOK(), "exit status coord")
	} else {
		log.Printf("COORD exited %v %v\n", err, string(status))
	}
}

func (ts *Tstate) testAbort() {
	b, err := ts.ReadFile(fn(ts.mfss[0], "x"))
	assert.Nil(ts.t, err, "ReadFile")
	assert.Equal(ts.t, b, []byte("x"))

	b, err = ts.ReadFile(fn(ts.mfss[2], "y"))
	assert.NotEqual(ts.t, nil, "ReadFile")
}

func (ts *Tstate) testCommit() {
	b, err := ts.ReadFile(fn(ts.mfss[0], "x"))
	assert.NotEqual(ts.t, nil, "ReadFile")

	b, err = ts.ReadFile(fn(ts.mfss[2], "y"))
	assert.Nil(ts.t, err, "ReadFile")
	assert.Equal(ts.t, b, []byte("y"))
}

func TestCommit(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	ts.setUpMemFSs(N)
	ts.setUpFiles(ts.mfss[0], ts.mfss[1])
	fws := ts.setUpParticipants("", N-1)

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "start")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit()

	ts.shutdown(fws)
}

func TestAbort(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	ts.setUpMemFSs(N)
	ts.setUpFiles(ts.mfss[0], ts.mfss[1])
	fws := ts.setUpParticipants("crash1", N-1)

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "start")

	time.Sleep(100 * time.Millisecond)

	ts.testAbort()

	ts.shutdown(fws)
}

func TestCrash2(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	ts.setUpMemFSs(N)
	ts.setUpFiles(ts.mfss[0], ts.mfss[1])
	fws := ts.setUpParticipants("", N-1)

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "crash2")

	time.Sleep(100 * time.Millisecond)

	ts.testAbort()

	// Run another 2PC to stop the participants

	ts.checkCoord(fws, "start")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit()

	ts.shutdown(fws)
}

func TestCrash3(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	ts.setUpMemFSs(N)
	ts.setUpFiles(ts.mfss[0], ts.mfss[1])
	fws := ts.setUpParticipants("", N-1)

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "crash3")
	args := []string{"restart"}
	args = append(args, fws...)
	gmcoord := groupmgr.Start(ts.System.FsLib, ts.System.ProcClnt, NCOORD, "bin/user/twopc-coord", args, 0)

	time.Sleep(100 * time.Millisecond)

	ts.testAbort()
	gmcoord.Stop()

	ts.shutdown(fws)
}

func TestCrash4(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	ts.setUpMemFSs(N)
	ts.setUpFiles(ts.mfss[0], ts.mfss[1])
	fws := ts.setUpParticipants("", N-1)

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws, "crash4")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit()

	ts.shutdown(fws)
}

func TestTwo(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	ts.setUpMemFSs(N)
	ts.setUpFiles(ts.mfss[0], ts.mfss[1])
	fws1 := ts.setUpParticipants("", N-1)

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws1, "")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit()

	ts.cleanup()

	// Run antoher 2PC

	fws2 := ts.setUpParticipants("", N-1)

	time.Sleep(500 * time.Millisecond)

	ts.checkCoord(fws2, "")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit()

	ts.cleanup()

	ts.shutdown(append(fws1, fws2...))
}
