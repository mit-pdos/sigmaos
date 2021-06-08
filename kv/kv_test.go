package kv

import (
	"log"
	"math/rand"
	// "regexp"
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

func (ts *Tstate) setup(nclerk int, memfs bool) string {
	// add 1 so that we can put to initialize
	mfs := ""
	if memfs {
		mfs = ts.spawnMemFS()
	} else {
		mfs = spawnKV(ts.fsl)
	}
	runBalancer(ts.fsl, "add", mfs)

	ts.clrks = make([]*KvClerk, nclerk)
	for i := 0; i < nclerk; i++ {
		ts.clrks[i] = MakeClerk()
	}

	if nclerk > 0 {
		for i := 0; i < NKEYS; i++ {
			err := ts.clrks[0].Put(key(i), key(i))
			assert.Nil(ts.t, err, "Put")
		}
	}
	return mfs
}

func ConcurN(t *testing.T, nclerk int) {
	const NMORE = 10

	ts := makeTstate(t)

	ts.mfss = append(ts.mfss, ts.setup(nclerk, true))

	ch := make(chan bool)
	for i := 0; i < nclerk; i++ {
		go ts.clerk(i, ch)
	}

	for s := 0; s < NMORE; s++ {
		ts.mfss = append(ts.mfss, ts.spawnMemFS())
		runBalancer(ts.fsl, "add", ts.mfss[len(ts.mfss)-1])
		// do some puts/gets
		time.Sleep(500 * time.Millisecond)
	}

	for s := 0; s < NMORE; s++ {
		runBalancer(ts.fsl, "del", ts.mfss[len(ts.mfss)-1])
		ts.stopMemFS(ts.mfss[len(ts.mfss)-1])
		ts.mfss = ts.mfss[0 : len(ts.mfss)-1]
		// do some puts/gets
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("Wait for clerks\n")

	for i := 0; i < nclerk; i++ {
		ch <- true
	}

	log.Printf("Done waiting for clerks\n")

	time.Sleep(100 * time.Millisecond)

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

// Zipfian:
// r := rand.New(rand.NewSource(time.Now().UnixNano()))
// z := rand.NewZipf(r, 2.0, 1.0, 100)
// z.Uint64()
//
func (ts *Tstate) clerkMon(c int, ch chan bool) {
	tot := int64(0)
	max := int64(0)
	n := int64(0)
	for true {
		k := rand.Intn(NKEYS)
		t0 := time.Now().UnixNano()
		v, err := ts.clrks[c].Get(key(k))
		t1 := time.Now().UnixNano()
		tot += t1 - t0
		if t1-t0 > max {
			max = t1 - t0
		}
		n += 1
		select {
		case <-ch:
			log.Printf("n %v avg %v ns max %v ns\n", n, tot/n, max)
			ts.clrks[c].Exit()
			return
		default:
			assert.Nil(ts.t, err, "Get "+key(k))
			assert.Equal(ts.t, key(k), v, "Get")
		}
	}
}

func readKVs(fsl *fslib.FsLib) *KvSet {
	for true {
		conf, err := readConfig(fsl, KVCONFIG)
		if err != nil {
			// balancer may be at work
			log.Printf("readKVs: err %v\n", err)
			time.Sleep(1000 * time.Millisecond)
			continue
		}
		kvs := makeKvs(conf.Shards)
		log.Printf("Monitor config %v\n", kvs)
		return kvs
	}
	return nil
}

func TestElastic(t *testing.T) {
	const S = 1000
	nclerk := 30
	ts := makeTstate(t)

	ts.setup(nclerk, false)

	for i := 0; i < 3000; i += S {
		// start out with no load, no growing/shrinking
		time.Sleep(S * time.Millisecond)
		kvs := readKVs(ts.fsl)
		assert.Equal(ts.t, 1, len(kvs.set), "No grow")
	}

	ch := make(chan bool)
	for i := 0; i < nclerk; i++ {
		go ts.clerkMon(i, ch)
	}

	// grow KV
	for i := 0; i < 10000; i += S {
		// start out with no load, no growing/shrinking
		time.Sleep(S * time.Millisecond)
	}
	kvs := readKVs(ts.fsl)
	assert.NotEqual(ts.t, 1, len(kvs.set), "Grow")

	for i := 0; i < nclerk; i++ {
		ch <- true
	}

	// shrink KV
	time.Sleep(5000 * time.Millisecond)

	kvs = readKVs(ts.fsl)
	assert.Equal(ts.t, 1, len(kvs.set), "Shrink")

	log.Printf("shutdown %v\n", kvs)

	memfs := kvs.first()

	ts.stopMemFS(memfs)

	ts.s.Shutdown(ts.fsl)
}
