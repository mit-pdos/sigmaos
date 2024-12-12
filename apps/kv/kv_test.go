package kv_test

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// "google.golang.org/protobuf/reflect/protoreflect"

	cproto "sigmaos/apps/cache/proto"

	"sigmaos/apps/cache"
	"sigmaos/apps/kv"
	"sigmaos/apps/kv/kvgrp"
	db "sigmaos/debug"
	"sigmaos/util/crash"
	"sigmaos/util/rand"
	// sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	NCLERK = 4

	CRASHBALANCER = 10000
	CRASHMOVER    = 1000
)

var balancerEv *crash.TeventMap
var moverEv *crash.TeventMap
var bothEv *crash.TeventMap

func init() {
	e0 := crash.NewEvent(crash.KVBALANCER_CRASH, CRASHBALANCER, 0.33)
	balancerEv = crash.NewTeventMapOne(e0)
	e1 := crash.NewEvent(crash.KVBALANCER_PARTITION, CRASHBALANCER, 0.5)
	balancerEv.Insert(e1)

	e0 = crash.NewEvent(crash.KVMOVER_CRASH, CRASHMOVER, 0.2)
	moverEv = crash.NewTeventMapOne(e0)
	e1 = crash.NewEventStartDelay(crash.KVMOVER_PARTITION, 1, 0, 2000, 0.5)
	moverEv.Insert(e1)

	bothEv = crash.NewTeventMap()
	bothEv.Merge(balancerEv)
	bothEv.Merge(moverEv)
}

func checkKvs(t *testing.T, kvs *kv.KvSet, n int) {
	for _, v := range kvs.Set {
		if v != n {
			assert.Equal(t, v, n+1, "checkKvs")
		}
	}
}

func TestCompile(t *testing.T) {
}

func TestBalance(t *testing.T) {
	conf := &kv.Config{}
	for i := 0; i < kv.NSHARD; i++ {
		conf.Shards = append(conf.Shards, "")
	}
	for k := 0; k < kv.NKVGRP; k++ {
		shards := kv.AddKv(conf, strconv.Itoa(k))
		conf.Shards = shards
		kvs := kv.NewKvs(conf.Shards)
		//db.DPrintf(db.ALWAYS, "balance %v %v\n", shards, kvs)
		checkKvs(t, kvs, kv.NSHARD/(k+1))
	}
	for k := kv.NKVGRP - 1; k > 0; k-- {
		shards := kv.DelKv(conf, strconv.Itoa(k))
		conf.Shards = shards
		kvs := kv.NewKvs(conf.Shards)
		//db.DPrintf(db.ALWAYS, "balance %v %v\n", shards, kvs)
		checkKvs(t, kvs, kv.NSHARD/k)
	}
}

func TestRegex(t *testing.T) {
	// grp re
	grpre := regexp.MustCompile(`group/grp-([0-9]+)-conf`)
	s := grpre.FindStringSubmatch("file not found group/grp-9-conf")
	assert.NotNil(t, s, "Find")
	s = grpre.FindStringSubmatch("file not found group/grp-10-conf")
	assert.NotNil(t, s, "Find")
	s = grpre.FindStringSubmatch("file not found group/grp-10-conf (no mount)")
	assert.NotNil(t, s, "Find")
	re := regexp.MustCompile(`grp-([0-9]+)`)
	s = re.FindStringSubmatch("grp-10")
	assert.NotNil(t, s, "Find")
}

type Tstate struct {
	*test.Tstate
	kvf *kv.KVFleet
	cm  *kv.ClerkMgr
	job string
}

func newTstate(t1 *test.Tstate, em *crash.TeventMap, auto string, repl int) *Tstate {
	ts := &Tstate{job: rand.String(4)}
	ts.Tstate = t1

	// XXX maybe in pe
	err := crash.SetSigmaFail(em)
	assert.Nil(t1.T, err)

	kvf, err := kv.NewKvdFleet(ts.SigmaClnt, ts.job, 1, repl, 0, auto)
	assert.Nil(t1.T, err)
	ts.kvf = kvf
	ts.cm, err = kv.NewClerkMgr(ts.SigmaClnt, ts.job, 0, repl > 0)
	assert.Nil(t1.T, err)
	err = ts.kvf.Start()
	assert.Nil(t1.T, err)
	err = ts.cm.StartCmClerk()
	assert.Nil(t1.T, err)
	err = ts.cm.InitKeys(kv.NKEYS)
	assert.Nil(t1.T, err)
	return ts
}

func (ts *Tstate) done() {
	db.DPrintf(db.TEST, "Stop Clerks")
	ts.cm.StopClerks()
	db.DPrintf(db.TEST, "Stop KVFleet")
	ts.kvf.Stop()
	ts.Shutdown()
}

func TestMiss(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nil, "manual", kv.KVD_NO_REPL)
	err := ts.cm.Get(cache.NewKey(kv.NKEYS+1), &cproto.CacheString{})
	assert.True(t, cache.IsMiss(err))
	ts.done()
}

func TestGetPut0(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nil, "manual", kv.KVD_NO_REPL)

	err := ts.cm.Get(cache.NewKey(kv.NKEYS+1), &cproto.CacheString{})
	assert.NotNil(ts.T, err, "Get")

	err = ts.cm.Put(cache.NewKey(kv.NKEYS+1), &cproto.CacheString{Val: ""})
	assert.Nil(ts.T, err, "Put")

	err = ts.cm.Put(cache.NewKey(0), &cproto.CacheString{Val: ""})
	assert.Nil(ts.T, err, "Put")

	for i := uint64(0); i < kv.NKEYS; i++ {
		key := cache.NewKey(i)
		err := ts.cm.Get(key, &cproto.CacheString{})
		assert.Nil(ts.T, err, "Get "+key)
	}

	ts.done()
}

func TestPutGetRepl(t *testing.T) {
	const TIME = 100

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nil, "manual", kv.KVD_REPL_LEVEL)

	err := ts.cm.StartClerks("", 1)
	assert.Nil(ts.T, err, "Error StartClerk: %v", err)

	start := time.Now()
	end := start.Add(10 * time.Second)
	for i := 0; start.Before(end); i++ {
		time.Sleep(TIME * time.Millisecond)
		start = time.Now()
	}
	db.DPrintf(db.TEST, "Done ")
	ts.done()
}

func TestPutGetCrashKVD1(t *testing.T) {
	const TIME = 100

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	e0 := crash.NewEvent(crash.KVD_CRASH, kvgrp.CRASH, 0.33)
	ts := newTstate(t1, crash.NewTeventMapOne(e0), "manual", kv.KVD_REPL_LEVEL)

	err := ts.cm.StartClerks("", 1)
	assert.Nil(ts.T, err, "Error StartClerk: %v", err)

	start := time.Now()
	end := start.Add(10 * time.Second)
	for i := 0; start.Before(end); i++ {
		time.Sleep(TIME * time.Millisecond)
		start = time.Now()
	}
	db.DPrintf(db.TEST, "Done ")
	ts.done()
}

func concurN(t *testing.T, nclerk int, em *crash.TeventMap, repl int) {
	const TIME = 100

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(t1, em, "manual", repl)

	err := ts.cm.StartClerks("", nclerk)
	assert.Nil(ts.T, err, "Error StartClerk: %v", err)

	db.DPrintf(db.TEST, "Done StartClerks")

	for i := 0; i < kv.NKVGRP; i++ {
		err := ts.kvf.AddKVDGroup()
		assert.Nil(ts.T, err, "AddKVDGroup")
		// allow some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	db.DPrintf(db.TEST, "Done adds")

	for i := 0; i < kv.NKVGRP; i++ {
		err := ts.kvf.RemoveKVDGroup()
		assert.Nil(ts.T, err, "RemoveKVDGroup")
		// allow some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	db.DPrintf(db.TEST, "Done dels")

	ts.cm.StopClerks()

	db.DPrintf(db.TEST, "Done stopClerks")

	time.Sleep(100 * time.Millisecond)

	err = ts.kvf.Stop()
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestKVOK0(t *testing.T) {
	concurN(t, 0, nil, kv.KVD_NO_REPL)
}

func TestKVOK1(t *testing.T) {
	concurN(t, 1, nil, kv.KVD_NO_REPL)
}

func TestKVOKN(t *testing.T) {
	concurN(t, NCLERK, nil, kv.KVD_NO_REPL)
}

func TestCrashBal0(t *testing.T) {
	concurN(t, 0, balancerEv, kv.KVD_NO_REPL)
}

func TestCrashBal1(t *testing.T) {
	concurN(t, 1, balancerEv, kv.KVD_NO_REPL)
}

func TestCrashBalN(t *testing.T) {
	concurN(t, NCLERK, balancerEv, kv.KVD_NO_REPL)
}

func TestCrashMov0(t *testing.T) {
	concurN(t, 0, moverEv, kv.KVD_NO_REPL)
}

func TestCrashMov1(t *testing.T) {
	concurN(t, 1, moverEv, kv.KVD_NO_REPL)
}

func TestCrashMovN(t *testing.T) {
	concurN(t, NCLERK, moverEv, kv.KVD_NO_REPL)
}

func TestCrashAll0(t *testing.T) {
	concurN(t, 0, bothEv, kv.KVD_NO_REPL)
}

func TestCrashAll1(t *testing.T) {
	concurN(t, 1, bothEv, kv.KVD_NO_REPL)
}

func TestCrashAllN(t *testing.T) {
	concurN(t, NCLERK, bothEv, kv.KVD_NO_REPL)
}

func TestReplOK0(t *testing.T) {
	concurN(t, 0, nil, kv.KVD_REPL_LEVEL)
}

func TestReplOK1(t *testing.T) {
	concurN(t, 1, nil, kv.KVD_REPL_LEVEL)
}

func TestReplOKN(t *testing.T) {
	concurN(t, NCLERK, nil, kv.KVD_REPL_LEVEL)
}

//
// Fix: repl crashing tests
//

func XTestReplCrash0(t *testing.T) {
	concurN(t, 0, nil, kv.KVD_REPL_LEVEL)
}

func XTestReplCrash1(t *testing.T) {
	concurN(t, 1, nil, kv.KVD_REPL_LEVEL)
}

func XTestReplCrashN(t *testing.T) {
	concurN(t, NCLERK, nil, kv.KVD_REPL_LEVEL)
}

func TestAuto(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(t1, nil, "auto", kv.KVD_NO_REPL)

	for i := 0; i < 0; i++ {
		err := ts.kvf.AddKVDGroup()
		assert.Nil(ts.T, err, "Error AddKVDGroup: %v", err)
	}

	err := ts.cm.StartClerks("10s", NCLERK)
	assert.Nil(ts.T, err, "Error StartClerks: %v", err)

	ts.cm.WaitForClerks()

	time.Sleep(100 * time.Millisecond)

	ts.kvf.Stop()

	ts.Shutdown()
}
