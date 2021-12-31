package kv

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/coordmgr"
	"ulambda/fslib"
	"ulambda/kernel"
)

const (
	NKEYS  = 100
	NCLERK = 10

	CRASHBALANCER = 400
)

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
	*kernel.System
	t     *testing.T
	clrks []*KvClerk
	mfss  []string
	cm    *coordmgr.CoordMgr
}

func makeTstate(t *testing.T, auto string, nclerk int, crash int) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemAll("kv_test", "..")
	ts.cm = coordmgr.StartCoords(ts.System.FsLib, ts.System.ProcClnt, "bin/user/balancer", []string{auto}, crash)

	ts.setup(nclerk)

	return ts
}

func (ts *Tstate) setup(nclerk int) {
	log.Printf("start kv\n")

	// add 1 kv so that we can put to initialize
	mfs := SpawnMemFS(ts.ProcClnt)
	err := ts.WaitStart(mfs)
	assert.Nil(ts.t, err, "Start mfs")

	err = ts.balancerOp("add", mfs)
	assert.Nil(ts.t, err, "BalancerOp")

	ts.clrks = make([]*KvClerk, nclerk)
	for i := 0; i < nclerk; i++ {
		ts.clrks[i] = MakeClerk(fslib.Named())
	}

	if nclerk > 0 {
		for i := uint64(0); i < NKEYS; i++ {
			err := ts.clrks[0].Put(key(i), key(i))
			assert.Nil(ts.t, err, "Put")
		}
	}
	ts.mfss = append(ts.mfss, mfs)
}

func (ts *Tstate) done() {
	ts.cm.StopCoords()
	ts.stopMemFSs()
	ts.Shutdown()
}

func (ts *Tstate) stopFS(fs string) {
	err := ts.Evict(fs)
	assert.Nil(ts.t, err, "ShutdownFS")
	ts.WaitExit(fs)
}

func (ts *Tstate) startMemFSs(n int) []string {
	mfss := make([]string, 0)
	for r := 0; r < n; r++ {
		mfs := SpawnMemFS(ts.ProcClnt)
		mfss = append(mfss, mfs)
	}
	return mfss
}

func (ts *Tstate) stopMemFSs() {
	for _, mfs := range ts.mfss {
		ts.stopFS(mfs)
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

func (ts *Tstate) balancerOp(opcode, mfs string) error {
	for true {
		err := BalancerOp(ts.FsLib, opcode, mfs)
		if err == nil {
			return err
		}
		// XXX error checking in one place and more uniform
		if err.Error() == "EOF" ||
			strings.HasPrefix(err.Error(), "file not found") ||
			strings.HasPrefix(err.Error(), "unable to connect") {
			time.Sleep(100 * time.Millisecond)
		} else {
			return err
		}
	}
	return nil
}

func TestGetPutSet(t *testing.T) {
	ts := makeTstate(t, "manual", 1, 0)

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

	ts.done()
}

func concurN(t *testing.T, nclerk int, crash int) {
	const NMORE = 10
	const TIME = 100 // 500

	ts := makeTstate(t, "manual", nclerk, crash)

	ch := make(chan bool)
	for i := 0; i < nclerk; i++ {
		go ts.clerk(i, ch)
	}

	for s := 0; s < NMORE; s++ {
		mfs := SpawnMemFS(ts.ProcClnt)
		ts.mfss = append(ts.mfss, mfs)
		ts.WaitStart(mfs)
		err := ts.balancerOp("add", ts.mfss[len(ts.mfss)-1])
		assert.Nil(ts.t, err, "BalancerOp")
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	for s := 0; s < NMORE; s++ {
		err := ts.balancerOp("del", ts.mfss[len(ts.mfss)-1])
		assert.Nil(ts.t, err, "BalancerOp")
		ts.stopFS(ts.mfss[len(ts.mfss)-1])
		ts.mfss = ts.mfss[0 : len(ts.mfss)-1]
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	log.Printf("Wait for clerks\n")

	for i := 0; i < nclerk; i++ {
		ch <- true
	}

	log.Printf("Done waiting for clerks\n")

	time.Sleep(100 * time.Millisecond)

	ts.cm.StopCoords()
	ts.stopMemFSs()

	ts.Shutdown()
}

func TestConcurOK0(t *testing.T) {
	concurN(t, 0, 0)
}

func TestConcurOK1(t *testing.T) {
	concurN(t, 1, 0)
}

func TestConcurOKN(t *testing.T) {
	concurN(t, NCLERK, 0)
}

func TestConcurFail0(t *testing.T) {
	concurN(t, 0, CRASHBALANCER)
}

func TestConcurFail1(t *testing.T) {
	concurN(t, 1, CRASHBALANCER)
}

func TestConcurFailN(t *testing.T) {
	concurN(t, NCLERK, CRASHBALANCER)
}

func TestAuto(t *testing.T) {
	// runtime.GOMAXPROCS(2) // XXX for KV
	nclerk := NCLERK
	ts := makeTstate(t, "auto", nclerk, 0)

	ch := make(chan bool)
	for i := 0; i < nclerk; i++ {
		go ts.clerk(i, ch)
	}

	time.Sleep(30 * time.Second)

	log.Printf("Wait for clerks\n")

	for i := 0; i < nclerk; i++ {
		ch <- true
	}

	ts.done()
}
