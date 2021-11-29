package main

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/realm"
	"ulambda/twopc"
)

type Tstate struct {
	t   *testing.T
	fsl *fslib.FsLib
	*procclnt.ProcClnt
	ch        chan bool
	chPresent chan bool
	mfss      []string
	pid       string
	e         *realm.TestEnv
	cfg       *realm.RealmConfig
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t

	ts.ch = make(chan bool)
	ts.chPresent = make(chan bool)

	bin := "../../../"
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	ts.fsl = fslib.MakeFsLibAddr("twopc_test", cfg.NamedAddr)
	ts.ProcClnt = procclnt.MakeProcClntInit(ts.fsl, cfg.NamedAddr)

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

	err = ts.fsl.Mkdir(named.MEMFS, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}

	return ts
}

func (ts *Tstate) waitParticipants(fws []string) {
	for _, pid := range fws {
		log.Printf("collect %v\n", pid)
		ts.WaitExit(pid)
	}
}

func (ts *Tstate) shutdown() {
	ts.stopMemFSs()
	ts.e.Shutdown()
}

func (ts *Tstate) cleanup() {
	err := ts.fsl.Remove(named.MEMFS + "/txni")
	assert.Nil(ts.t, err, "Remove txni")
}

func (ts *Tstate) spawnMemFS() string {
	p := proc.MakeProc("bin/user/memfsd", []string{""})
	ts.Spawn(p)
	ts.WaitStart(p.Pid)
	return p.Pid
}

func (ts *Tstate) spawnParticipant(index, opcode, delay string) string {
	p := proc.MakeProc("bin/user/test2pc2", []string{index, opcode, delay})
	ts.Spawn(p)
	ts.WaitStart(p.Pid)
	return p.Pid
}

func (ts *Tstate) spawnCoord(opcode string, fws []string) string {
	p := proc.MakeProc("bin/user/twopc-coord", append([]string{opcode}, fws...))
	ts.Spawn(p)
	ts.WaitStart(p.Pid)
	return p.Pid
}

func (ts *Tstate) startParticipants(n int, opcode string, delays []string) []string {
	fws := make([]string, 0)
	for r := 0; r < n; r++ {
		var fw string
		if opcode != "" && r == 0 {
			fw = ts.spawnParticipant(strconv.Itoa(r), opcode, delays[r])
		} else {
			fw = ts.spawnParticipant(strconv.Itoa(r), "", delays[r])
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
		err := ts.fsl.ShutdownFs(named.MEMFS + "/" + mfs)
		assert.Nil(ts.t, err, "Remove")
		ts.WaitExit(mfs)
	}
}

func fn(mfs, f string) string {
	return named.MEMFS + "/" + mfs + "/" + f
}

func (ts *Tstate) setUpMemFSs(N int) {
	ts.mfss = ts.startMemFSs(N)
}

func (ts *Tstate) setUpFiles(nPart int) {
	for i := 0; i < nPart; i++ {
		iStr := strconv.Itoa(i)
		err := ts.fsl.MakeFile(fn(ts.mfss[0], iStr), 0777, np.OWRITE, []byte(iStr))
		assert.Nil(ts.t, err, "MakeFile")
	}
}

func (ts *Tstate) setUpParticipants(opcode string, val string, delays []string, N int) []string {
	ti := Tinput{}
	for i := 0; i < N; i++ {
		iStr := strconv.Itoa(i)
		ti.Fns = append(ti.Fns, fn(ts.mfss[0], iStr))
		ti.Vals = append(ti.Vals, val)
	}

	err := ts.fsl.MakeFileJson(named.MEMFS+"/txni", 0777, ti)
	assert.Nil(ts.t, err, "MakeFile")

	fws := ts.startParticipants(N, opcode, delays)
	return fws
}

func (ts *Tstate) checkCoord(fws []string, opcode string) {
	pid := ts.spawnCoord(opcode, fws)
	ok, err := ts.WaitExit(pid)
	assert.Nil(ts.t, err, "WaitStart")
	if !strings.HasPrefix(opcode, "crash") {
		assert.Equal(ts.t, "OK", string(ok))
	} else {
		log.Printf("COORD exited %v %v\n", err, string(ok))
	}
}

func (ts *Tstate) testAbort(N int) {
	for i := 0; i < N; i++ {
		iStr := strconv.Itoa(i)
		b, err := ts.fsl.ReadFile(fn(ts.mfss[0], iStr))
		assert.Nil(ts.t, err, "ReadFile")
		assert.Equal(ts.t, iStr, string(b), "Equal")
	}
}

func (ts *Tstate) testCommit(N int) {
	var prev string
	var cur string
	for i := 0; i < N; i++ {
		iStr := strconv.Itoa(i)
		b, err := ts.fsl.ReadFile(fn(ts.mfss[0], iStr))
		assert.Nil(ts.t, err, "ReadFile")
		cur = string(b)
		if i > 0 {
			assert.Equal(ts.t, prev, cur, "Equal")
		}
		prev = cur
	}
}

func TestCommit(t *testing.T) {
	ts := makeTstate(t)
	const N_MEMFS = 1
	const N_PART = 2
	ts.setUpMemFSs(N_MEMFS)
	ts.setUpFiles(N_PART)

	var delays []string
	for i := 0; i < N_PART; i++ {
		dur := 0 * time.Second
		delays = append(delays, dur.String())
	}

	fws := ts.setUpParticipants("", "one", delays, N_PART)

	ts.checkCoord(fws, "start")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit(N_PART)

	ts.waitParticipants(fws)
	ts.shutdown()
}

func TestAbort(t *testing.T) {
	ts := makeTstate(t)
	const N_MEMFS = 1
	const N_PART = 2
	ts.setUpMemFSs(N_MEMFS)
	ts.setUpFiles(N_PART)

	var delays []string
	for i := 0; i < N_PART; i++ {
		dur := 0 * time.Second
		delays = append(delays, dur.String())
	}

	fws := ts.setUpParticipants("crash1", "one", delays, N_PART)

	ts.checkCoord(fws, "start")

	time.Sleep(100 * time.Millisecond)

	ts.testAbort(N_PART)

	ts.waitParticipants(fws)
	ts.shutdown()
}

func TestTwo(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	const N_MEMFS = 1
	const N_PART = 2
	ts.setUpMemFSs(N_MEMFS)
	ts.setUpFiles(N_PART)

	var delays1 []string
	for i := 0; i < N_PART; i++ {
		dur := 0 * time.Second
		delays1 = append(delays1, dur.String())
	}

	fws1 := ts.setUpParticipants("", "one", delays1, N_PART)

	ts.checkCoord(fws1, "")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit(N_PART)

	ts.cleanup()

	// Run antoher 2PC

	var delays2 []string
	for i := 0; i < N_PART; i++ {
		dur := 0 * time.Second
		delays2 = append(delays2, dur.String())
	}

	fws2 := ts.setUpParticipants("", "one", delays2, N_PART)

	ts.checkCoord(fws2, "")

	time.Sleep(100 * time.Millisecond)

	ts.testCommit(N_PART)

	ts.cleanup()

	ts.waitParticipants(append(fws1, fws2...))
	ts.shutdown()
}

func TestTwoConcurrent(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	const N_MEMFS = 1
	const N_PART = 2
	ts.setUpMemFSs(N_MEMFS)
	ts.setUpFiles(N_PART)

	var delays1 []string
	for i := 0; i < N_PART; i++ {
		dur := 0 * time.Second
		delays1 = append(delays1, dur.String())
	}

	fws1 := ts.setUpParticipants("", "one", delays1, N_PART)

	done1 := make(chan bool)
	go func() {
		ts.checkCoord(fws1, "")

		time.Sleep(100 * time.Millisecond)

		ts.testCommit(N_PART)
		done1 <- true
	}()

	time.Sleep(200 * time.Millisecond)

	ts.cleanup()

	// Run antoher 2PC

	var delays2 []string
	for i := 0; i < N_PART; i++ {
		dur := 0 * time.Second
		delays2 = append(delays2, dur.String())
	}

	fws2 := ts.setUpParticipants("", "two", delays2, N_PART)

	done2 := make(chan bool)
	go func() {
		ts.checkCoord(fws2, "")

		time.Sleep(100 * time.Millisecond)

		ts.testCommit(N_PART)

		done2 <- true
	}()

	<-done1
	<-done2

	ts.cleanup()

	ts.waitParticipants(append(fws1, fws2...))
	ts.shutdown()
}

func TestTwoConcurrentCrashCoord(t *testing.T) {
	ts := makeTstate(t)
	const N = 3
	const N_MEMFS = 1
	const N_PART = 2
	ts.setUpMemFSs(N_MEMFS)
	ts.setUpFiles(N_PART)

	var delays1 []string
	for i := 0; i < N_PART; i++ {
		dur := 1 * time.Second
		delays1 = append(delays1, dur.String())
	}

	fws1 := ts.setUpParticipants("delayCommit", "one", delays1, N_PART)

	done1 := make(chan bool)
	go func() {
		ts.checkCoord(fws1, "crash4")

		time.Sleep(100 * time.Millisecond)

		done1 <- true
	}()

	time.Sleep(200 * time.Millisecond)

	ts.cleanup()

	// Run antoher 2PC

	var delays2 []string
	for i := 0; i < N_PART; i++ {
		dur := 0 * time.Second
		delays2 = append(delays2, dur.String())
	}

	fws2 := ts.setUpParticipants("", "two", delays2, N_PART)

	done2 := make(chan bool)
	go func() {
		ts.checkCoord(fws2, "")

		time.Sleep(100 * time.Millisecond)

		done2 <- true
	}()

	<-done1
	<-done2

	ts.cleanup()

	ts.waitParticipants(append(fws1, fws2...))
	ts.testCommit(N_PART)
	ts.shutdown()
}
