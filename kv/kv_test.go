package kv

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
)

const NKEYS = 100
const NCLERK = 10

type Tstate struct {
	t         *testing.T
	s         *fslib.System
	fsl       *fslib.FsLib
	clrks     []*KvClerk
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
	ts.fsl = fslib.MakeFsLib("kv_test")

	// Setup KV configuration: name/kv, name/kv/commit/, and
	// initial name/kv/config
	err = ts.fsl.Mkdir("name/kv", 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	err = ts.fsl.Mkdir(KVPREPARED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", KVPREPARED, err)
	}
	conf := makeConfig(0)
	err = ts.fsl.MakeFileJson(KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", KVCONFIG, err)
	}

	ts.pid = ts.makeKV()

	ts.clrks = make([]*KvClerk, NCLERK)
	for i := 0; i < NCLERK; i++ {
		ts.clrks[i] = MakeClerk()
	}

	return ts
}

func (ts *Tstate) spawnKv(arg string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/kvd"
	a.Args = []string{arg}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
	return a.Pid
}

func (ts *Tstate) makeKV() string {
	pid := ts.spawnKv("add")
	if ok := ts.waitUntilPresent(kvname(pid)); !ok {
		log.Fatalf("Couldn't add first KV\n")
	}
	return pid
}

func (ts *Tstate) spawnSharder(opcode, pid string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/sharderd"
	a.Args = []string{opcode, pid}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
	return a.Pid
}

func (ts *Tstate) presentWatch(p string) {
	log.Printf("KV present watch fires")
	ts.chPresent <- true
}

func (ts *Tstate) waitUntilPresent(kv string) bool {
	conf := Config{}
	for {
		err := ts.fsl.ReadFileJsonWatch(KVCONFIG, &conf, ts.presentWatch)
		if err == nil {
			if conf.present(kv) {
				return true
			}
		} else if strings.HasPrefix(err.Error(), "file not found") {
			<-ts.chPresent
		} else {
			break
		}
	}
	return false
}

func (ts *Tstate) delFirst() {
	pid1 := ts.spawnSharder("del", kvname(ts.pid))
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(ts.t, err, "Wait")
	assert.Equal(ts.t, string(ok), "OK")
	time.Sleep(200 * time.Millisecond)
}

func key(k int) string {
	return "key" + strconv.Itoa(k)
}

func (ts *Tstate) getKeys(c int) bool {
	for i := 0; i < NKEYS; i++ {
		v, err := ts.clrks[c].Get(key(i))
		select {
		case <-ts.ch:
			return true
		default:
			assert.Nil(ts.t, err, "Get "+key(i))
			assert.Equal(ts.t, key(i), v, "Get")
		}
	}
	return false
}

func (ts *Tstate) clerk(c int) {
	done := false
	for !done {
		done = ts.getKeys(c)
	}
	assert.NotEqual(ts.t, 0, ts.clrks[c].nget)
}

func (ts *Tstate) startKVs(n int) []string {
	pids := make([]string, 0)
	for r := 0; r < n; r++ {
		pid := ts.makeKV()
		log.Printf("Added %v\n", pid)
		pids = append(pids, pid)
		time.Sleep(200 * time.Millisecond)
	}
	return pids
}

func (ts *Tstate) stopKVs(pids []string) {
	for _, pid := range pids {
		log.Printf("Del %v\n", pid)
		pid1 := ts.spawnSharder("del", kvname(pid))
		ok, err := ts.fsl.Wait(pid1)
		assert.Nil(ts.t, err, "Wait")
		assert.Equal(ts.t, string(ok), "OK")
		time.Sleep(200 * time.Millisecond)
	}
}

func ConcurN(t *testing.T, nclerk int) {
	ts := makeTstate(t)

	for i := 0; i < NKEYS; i++ {
		err := ts.clrks[0].Put(key(i), key(i))
		assert.Nil(t, err, "Put")
	}

	for i := 0; i < nclerk; i++ {
		go ts.clerk(i)
	}

	pids := ts.startKVs(NSHARD - 1)
	ts.stopKVs(pids)

	for i := 0; i < nclerk; i++ {
		ts.ch <- true
	}

	ts.delFirst()

	ts.s.Shutdown(ts.fsl)
}

func TestConcur0(t *testing.T) {
	ConcurN(t, 1)
}

func TestConcur1(t *testing.T) {
	ConcurN(t, 1)
}

func TestConcurN(t *testing.T) {
	ConcurN(t, NCLERK)
}

func (ts *Tstate) runSharder(t *testing.T) {
	pid1 := ts.spawnSharder("restart", "")
	log.Printf("sharder spawned %v\n", pid1)
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, string(ok), "OK")
	log.Printf("sharder %v done\n", pid1)
}

func TestConcurSharder(t *testing.T) {
	const N = 5

	ts := makeTstate(t)

	for r := 0; r < N; r++ {
		go ts.runSharder(t)
	}
	ts.s.Shutdown(ts.fsl)
}

func TestCrashSharder(t *testing.T) {
	const N = 1
	ts := makeTstate(t)

	pids := ts.startKVs(N)

	pid := ts.spawnKv("crash1")

	time.Sleep(1000 * time.Millisecond)

	log.Printf("sharder crashed\n")

	pid1 := ts.spawnSharder("restart", kvname(pid))
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, string(ok), "OK")

	log.Printf("SHARDER restart done\n")

	pid = ts.spawnKv("crash2")

	time.Sleep(1000 * time.Millisecond)

	log.Printf("sharder crashed\n")

	pid1 = ts.spawnSharder("restart", kvname(pid))
	ok, err = ts.fsl.Wait(pid1)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, string(ok), "OK")

	log.Printf("SHARDER restart done\n")

	ts.stopKVs(pids)
	ts.delFirst()

	ts.s.Shutdown(ts.fsl)
}
