package kv

import (
	"log"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/depproc"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/memfsd"
	"ulambda/proc"
	"ulambda/procinit"
)

const NKEYS = 2 // 100
const NCLERK = 10

func TestBalance(t *testing.T) {
	conf := &Config{}
	for i := 0; i < NSHARD; i++ {
		conf.Shards = append(conf.Shards, "")
	}
	shards := balanceAdd(conf, "a")
	log.Printf("balance %v\n", shards)
	conf.Shards = shards
	shards = balanceAdd(conf, "b")
	log.Printf("balance %v\n", shards)
	conf.Shards = shards
	shards = balanceAdd(conf, "c")
	log.Printf("balance %v\n", shards)
	conf.Shards = shards
	shards = balanceDel(conf, "c")
	log.Printf("balance %v\n", shards)
}

type Tstate struct {
	t   *testing.T
	s   *kernel.System
	fsl *fslib.FsLib
	proc.ProcCtl
	clrks []*KvClerk
	mfss  []string
	rand  *rand.Rand
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t

	procinit.SetProcLayers(map[string]bool{procinit.BASEPROC: true, procinit.DEPPROC: true})

	s, err := kernel.Boot("..")
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	ts.fsl = fslib.MakeFsLib("kv_test")
	ts.ProcCtl = procinit.MakeProcCtl(ts.fsl, procinit.GetProcLayersMap())

	err = ts.fsl.Mkdir(memfsd.MEMFS, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	err = ts.fsl.Mkdir(KVDIR, 07)
	if err != nil {
		t.Fatalf("Mkdir kv %v\n", err)
	}
	conf := MakeConfig(0)
	err = ts.fsl.MakeFileJson(KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", KVCONFIG, err)
	}
	ts.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return ts
}

func (ts *Tstate) spawnMemFS() string {
	t := depproc.MakeDepProc()
	a := &proc.Proc{}
	a.Pid = proc.GenPid()
	a.Program = "bin/kernel/memfsd"
	a.Args = []string{""}
	a.Env = []string{procinit.GetProcLayersString()}
	t.Proc = a
	ts.Spawn(t)
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

func key(k uint64) string {
	return "key" + strconv.FormatUint(k, 16)
}

func (ts *Tstate) getKeys(c int, ch chan bool) bool {
	for i := uint64(0); i < NKEYS; i++ {
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
		mfs = SpawnKV(ts.ProcCtl)
	}
	RunBalancer(ts.ProcCtl, "add", mfs)

	ts.clrks = make([]*KvClerk, nclerk)
	for i := 0; i < nclerk; i++ {
		ts.clrks[i] = MakeClerk()
	}

	if nclerk > 0 {
		for i := uint64(0); i < NKEYS; i++ {
			err := ts.clrks[0].Put(key(i), key(i))
			assert.Nil(ts.t, err, "Put")
		}
	}
	return mfs
}

func TestGetPutSet(t *testing.T) {
	ts := makeTstate(t)
	ts.mfss = append(ts.mfss, ts.setup(1, true))

	_, err := ts.clrks[0].Get(key(NKEYS + 1))
	assert.NotEqual(ts.t, err, nil, "Get")

	err = ts.clrks[0].Set(key(NKEYS+1), key(NKEYS+1))
	assert.NotEqual(ts.t, err, nil, "Set")

	err = ts.clrks[0].Set(key(0), key(0))
	assert.Nil(ts.t, err, "Set")

	for i := uint64(0); i < NKEYS; i++ {
		v, err := ts.clrks[0].Get(key(i))
		assert.Nil(ts.t, err, "Get "+key(i))
		assert.Equal(ts.t, key(i), v, "Get")
	}

	ts.stopMemFSs()
	ts.s.Shutdown(ts.fsl)
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
		RunBalancer(ts.ProcCtl, "add", ts.mfss[len(ts.mfss)-1])
		// do some puts/gets
		time.Sleep(500 * time.Millisecond)
	}

	for s := 0; s < NMORE; s++ {
		RunBalancer(ts.ProcCtl, "del", ts.mfss[len(ts.mfss)-1])
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
