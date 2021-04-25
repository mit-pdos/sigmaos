package test2pc

import (
	"log"
	"strconv"
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

func (ts *Tstate) spawnFlwr(index string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/test2pc"
	a.Args = []string{index, ""}
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

func (ts *Tstate) startFlwrs(n int) []string {
	fws := make([]string, 0)
	for r := 0; r < n; r++ {
		fw := ts.spawnFlwr(strconv.Itoa(r))
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
	ti.Fns = []string{
		fn(mfss[0], ""),
		fn(mfss[1], ""),
		fn(mfss[2], ""),
	}

	err = ts.fsl.MakeFileJson(memfsd.MEMFS+"/txni", 0777, ti)
	assert.Nil(t, err, "MakeFile")

	fws := ts.startFlwrs(N - 1)

	time.Sleep(500 * time.Millisecond)

	pid := twopc.SpawnCoord(ts.fsl, "start", fws)
	ok, err := ts.fsl.Wait(pid)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, string(ok), "OK")

	b, err := ts.fsl.ReadFile(fn(mfss[2], "y"))
	assert.Nil(t, err, "ReadFile")
	assert.Equal(t, b, []byte("y"))

	time.Sleep(100 * time.Millisecond)

	ts.stopMemFSs(mfss)

	ts.s.Shutdown(ts.fsl)
}
