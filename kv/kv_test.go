package kv

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
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
	err = ts.fsl.Mkdir(KVCOMMITTED, 0777)
	if err != nil {
		t.Fatalf("MkDir %v failed %v\n", KVCOMMITTED, err)
	}
	conf := makeConfig(0)
	err = ts.fsl.MakeFileJson(KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", KVCONFIG, err)
	}

	ts.pid = ts.makeKV()

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

func (ts *Tstate) presentWatch(p string, err error) {
	db.DLPrintf("KV", "presentWatch fires %v %v", p, err)
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
			time.Sleep(100 * time.Millisecond)
		} else if strings.HasPrefix(err.Error(), "file not found") {
			<-ts.chPresent
		} else {
			break
		}
	}
	return false
}

func (ts *Tstate) spawnCoord(opcode, pid string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/coordd"
	a.Args = []string{opcode, pid}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
	return a.Pid
}

func (ts *Tstate) delFirst() {
	log.Printf("Del first %v\n", kvname(ts.pid))
	pid1 := ts.spawnCoord("del", kvname(ts.pid))
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(ts.t, err, "Wait")
	assert.Equal(ts.t, "OK", string(ok))
	time.Sleep(200 * time.Millisecond)
}

func (ts *Tstate) runCoord(t *testing.T, ch chan bool) {
	pid1 := ts.spawnCoord("restart", "")
	log.Printf("coord spawned %v\n", pid1)
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(t, err, "Wait")
	assert.Equal(t, "OK", string(ok))
	log.Printf("coord %v done\n", pid1)
	ch <- true
}

func TestConcurCoord(t *testing.T) {
	const N = 5

	ts := makeTstate(t)
	ch := make(chan bool)
	for r := 0; r < N; r++ {
		go ts.runCoord(t, ch)
	}
	for r := 0; r < N; r++ {
		<-ch
	}
	ts.delFirst()
	ts.s.Shutdown(ts.fsl)
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

// start n more KVs (beyond the one made in makeTstate())
func (ts *Tstate) startKVs(n int, clerks bool) []string {
	pids := make([]string, 0)
	for r := 0; r < n; r++ {
		pid := ts.makeKV()
		log.Printf("Added %v\n", pid)
		pids = append(pids, pid)
		if clerks {
			// To allow clerk to do some gets:
			time.Sleep(200 * time.Millisecond)
		}
	}
	return pids
}

func (ts *Tstate) stopKVs(pids []string, clerks bool) {
	for _, pid := range pids {
		log.Printf("Del %v\n", pid)
		pid1 := ts.spawnCoord("del", kvname(pid))
		ok, err := ts.fsl.Wait(pid1)
		assert.Nil(ts.t, err, "Wait")
		assert.Equal(ts.t, "OK", string(ok))
		if clerks {
			// To allow clerk to do some gets:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func ConcurN(t *testing.T, nclerk int) {
	ts := makeTstate(t)

	ts.clrks = make([]*KvClerk, nclerk)
	for i := 0; i < nclerk; i++ {
		ts.clrks[i] = MakeClerk()
	}

	if nclerk > 0 {
		for i := 0; i < NKEYS; i++ {
			err := ts.clrks[0].Put(key(i), key(i))
			assert.Nil(t, err, "Put")
		}
	}

	for i := 0; i < nclerk; i++ {
		go ts.clerk(i)
	}

	pids := ts.startKVs(NSHARD-1, nclerk > 0)
	ts.stopKVs(pids, nclerk > 0)

	log.Printf("Wait for clerks\n")

	for i := 0; i < nclerk; i++ {
		ts.ch <- true
	}

	log.Printf("Done waiting for clerks\n")

	ts.delFirst()

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

func (ts *Tstate) restart(pid string) {
	pid1 := ts.spawnCoord("restart", kvname(pid))
	ok, err := ts.fsl.Wait(pid1)
	assert.Nil(ts.t, err, "Wait")
	assert.Equal(ts.t, "OK", string(ok))
	log.Printf("COORD restart done\n")
}

func TestCrashCoord(t *testing.T) {
	const NMORE = 1
	ts := makeTstate(t)

	pids := ts.startKVs(NMORE, false)

	// XXX fix exit status of coord in KV
	pid := ts.spawnKv("crash1")
	_, err := ts.fsl.Wait(pid)
	assert.Nil(ts.t, err, "Wait")

	time.Sleep(1000 * time.Millisecond)

	// see if we can add a new KV
	pid = ts.makeKV()
	log.Printf("Added %v\n", pid)
	pids = append(pids, pid)

	pid = ts.spawnKv("crash2")
	pids = append(pids, pid) // all KVs will prepare

	// XXX wait until new KVCONFIG exists with pid
	time.Sleep(1000 * time.Millisecond)

	// see if we can add a new KV
	pid = ts.makeKV()
	log.Printf("Added %v\n", pid)
	pids = append(pids, pid)

	// coord crashes after adding new KV
	pid = ts.spawnKv("crash3")
	ok := ts.waitUntilPresent(kvname(pid))
	assert.Equal(ts.t, true, ok)
	pids = append(pids, pid)

	ts.stopKVs(pids, false)
	ts.delFirst()

	ts.s.Shutdown(ts.fsl)
}

func TestCrashKV(t *testing.T) {
	const NMORE = 1
	ts := makeTstate(t)

	pids := ts.startKVs(NMORE, false)

	pid := ts.spawnKv("crash4")
	_, err := ts.fsl.Wait(pid)
	assert.Nil(ts.t, err, "Wait")

	// XXX wait until new KVCONFIG exists with pid
	time.Sleep(1000 * time.Millisecond)

	// see if we can add a new KV
	pid = ts.makeKV()
	log.Printf("Added %v\n", pid)
	pids = append(pids, pid)

	pid = ts.spawnKv("crash5")
	_, err = ts.fsl.Wait(pid)
	assert.Nil(ts.t, err, "Wait")

	// forceful remove pid, since it has crashed
	pid1 := ts.spawnCoord("excl", kvname(pid))
	_, err = ts.fsl.Wait(pid1)
	assert.Nil(ts.t, err, "Wait")

	ts.stopKVs(pids, false)
	ts.delFirst()

	ts.s.Shutdown(ts.fsl)
}
