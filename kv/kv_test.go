package kv

import (
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
)

const NKEYS = 100
const NCLERK = 10

type Tstate struct {
	t     *testing.T
	s     *fslib.System
	fsl   *fslib.FsLib
	clrks []*KvClerk
	mfss  []string
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t

	s, err := fslib.Boot("..")
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	ts.fsl = fslib.MakeFsLib("kv_test")

	err = ts.fsl.Mkdir(memfsd.MEMFS, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	err = ts.fsl.Mkdir(KVDIR, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	conf := makeConfig(0)
	err = ts.fsl.MakeFileJson(KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", KVCONFIG, err)
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

func (ts *Tstate) startMemFSs(n int) []string {
	mfss := make([]string, 0)
	for r := 0; r < n; r++ {
		mfs := ts.spawnMemFS()
		mfss = append(mfss, mfs)
	}
	return mfss
}

func (ts *Tstate) stopMemFS(mfs string) {
	err := ts.fsl.Remove(memfsd.MEMFS + "/" + mfs + "/")
	assert.Nil(ts.t, err, "Remove")
}

func (ts *Tstate) stopMemFSs() {
	for _, mfs := range ts.mfss {
		ts.stopMemFS(mfs)
	}
}

func (ts *Tstate) spawnBalancer(opcode, mfs string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/balancer"
	a.Args = []string{opcode, mfs}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
	return a.Pid
}

func (ts *Tstate) runBalancer(opcode, mfs string) {
	pid1 := ts.spawnBalancer(opcode, mfs)
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(ts.t, err, "Wait")
	assert.Equal(ts.t, "OK", string(ok))
	log.Printf("balancer %v done\n", pid1)
}

func key(k int) string {
	return "key" + strconv.Itoa(k)
}

func (ts *Tstate) getKeys(c int, ch chan bool) bool {
	for i := 0; i < NKEYS; i++ {
		v, err := ts.clrks[c].Get(key(i))
		select {
		case <-ch:
			return true
		default:
			assert.Nil(ts.t, err, "Get "+key(i))
			assert.Equal(ts.t, key(i), v, "Get")
		}
	}
	return false
}

func (ts *Tstate) clerk(c int, ch chan bool) {
	done := false
	for !done {
		done = ts.getKeys(c, ch)
	}
	log.Printf("nget %v\n", ts.clrks[c].nget)
	assert.NotEqual(ts.t, 0, ts.clrks[c].nget)
}

func ConcurN(t *testing.T, nclerk int) {
	const NMORE = 2

	ts := makeTstate(t)

	// add 1 so that we can put to initialize
	ts.mfss = append(ts.mfss, ts.spawnMemFS())
	ts.runBalancer("add", ts.mfss[0])

	ts.clrks = make([]*KvClerk, nclerk)
	for i := 0; i < nclerk; i++ {
		ts.clrks[i] = MakeClerk()
	}

	if nclerk > 0 {
		log.Printf("make keys\n")
		for i := 0; i < NKEYS; i++ {
			err := ts.clrks[0].Put(key(i), key(i))
			assert.Nil(t, err, "Put")
		}
	}

	ch := make(chan bool)
	for i := 0; i < nclerk; i++ {
		go ts.clerk(i, ch)
	}

	for s := 0; s < NMORE; s++ {
		ts.mfss = append(ts.mfss, ts.spawnMemFS())
		ts.runBalancer("add", ts.mfss[len(ts.mfss)-1])
		// do some puts/gets
		time.Sleep(1000 * time.Millisecond)
	}

	log.Printf("Wait for clerks\n")

	for i := 0; i < nclerk; i++ {
		ch <- true
	}

	log.Printf("Done waiting for clerks\n")

	time.Sleep(100000 * time.Millisecond)

	ts.stopMemFSs()

	ts.s.Shutdown(ts.fsl)
}

func TestConcur0(t *testing.T) {
	ConcurN(t, 0)
}

func TestConcur1(t *testing.T) {
	ConcurN(t, 1)
}

func TestConcurN(t *testing.T) {
	ConcurN(t, NCLERK)
}
