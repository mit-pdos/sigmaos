package twopc

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
)

type Tstate struct {
	t         *testing.T
	s         *fslib.System
	fsl       *fslib.FsLib
	ch        chan bool
	chPresent chan bool
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

	err = ts.fsl.Mkdir(TXNDIR, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	err = ts.fsl.Mkdir(TXNPREPARED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", TXNPREPARED, err)
	}
	err = ts.fsl.Mkdir(TXNCOMMITTED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", TXNCOMMITTED, err)
	}

	err = ts.fsl.Mkdir(memfsd.MEMFS, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}

	return ts
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

func (ts *Tstate) spawnFlwr() string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/flwr"
	a.Args = []string{""}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
	return a.Pid
}

func (ts *Tstate) runCoord(t *testing.T, ch chan bool) {
	pid1 := spawnCoord(ts.fsl, "restart", "", nil)
	log.Printf("coord spawned %v\n", pid1)
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, string(ok), "OK")
	ch <- true
}

func xTestConcurCoord(t *testing.T) {
	const N = 5

	ts := makeTstate(t)
	ch := make(chan bool)
	for r := 0; r < N; r++ {
		go ts.runCoord(t, ch)
	}
	for r := 0; r < N; r++ {
		<-ch
	}
	ts.s.Shutdown(ts.fsl)
}

func (ts *Tstate) startFlwrs(n int) []string {
	fws := make([]string, 0)
	for r := 0; r < n; r++ {
		fw := ts.spawnFlwr()
		fws = append(fws, flwname(fw))
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

func (ts *Tstate) stopMemFSs(mfss []string) {
	for _, mfs := range mfss {
		err := ts.fsl.Remove(memfsd.MEMFS + "/" + mfs + "/")
		assert.Nil(ts.t, err, "Remove")
	}
}

func fn(mfs, f string) string {
	return memfsd.MEMFS + "/" + mfs + "/" + f
}

func TestTwoPC(t *testing.T) {
	const N = 3

	ts := makeTstate(t)

	mfss := ts.startMemFSs(N)

	time.Sleep(200 * time.Millisecond)

	err := ts.fsl.MakeFile(fn(mfss[0], "x"), 0777, []byte("x"))
	assert.Nil(t, err, "MakeFile")
	err = ts.fsl.MakeFile(fn(mfss[1], "y"), 0777, []byte("y"))
	assert.Nil(t, err, "MakeFile")

	ti := Tinput{}
	ti.Fns = []string{fn(mfss[0], "x"), fn(mfss[1], "y")}

	err = ts.fsl.MakeFileJson(memfsd.MEMFS+"/txni", 0777, ti)
	assert.Nil(t, err, "MakeFile")

	fws := ts.startFlwrs(N - 1)

	time.Sleep(500 * time.Millisecond)

	pid := spawnCoord(ts.fsl, "start", "bin/testtxn", fws)
	ok, err := ts.fsl.Wait(pid)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, string(ok), "OK")

	time.Sleep(100 * time.Millisecond)

	ts.stopMemFSs(mfss)

	ts.s.Shutdown(ts.fsl)
}
