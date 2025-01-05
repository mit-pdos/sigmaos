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

	PARTITION = 200

	CRASHBALANCER   = 4000
	CRASHMOVERDELAY = -10
)

var balancerEv *crash.TeventMap
var moverEv *crash.TeventMap
var bothEv *crash.TeventMap
var EvP = crash.NewEvent(crash.KVD_PARTITION, PARTITION, 0.33)

func init() {
	e0 := crash.NewEvent(crash.KVBALANCER_CRASH, CRASHBALANCER, 0.2)
	balancerEv = crash.NewTeventMapOne(e0)
	e1 := crash.NewEvent(crash.KVBALANCER_PARTITION, CRASHBALANCER, 0.2)
	balancerEv.Insert(e1)

	e0 = crash.NewEvent(crash.KVMOVER_EVENT, CRASHMOVERDELAY, 0.2)
	moverEv = crash.NewTeventMapOne(e0)

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

func concurN(t *testing.T, nclerk int, em *crash.TeventMap, repl int) (int, int, kv.TclerkRes) {
	const TIME = 100

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return 0, 0, kv.TclerkRes{}
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

	cr, err := ts.cm.StopClerks()

	assert.Nil(t, err)
	assert.True(t, cr.Nkeys >= int64(nclerk*kv.NKEYS))

	db.DPrintf(db.TEST, "Done stopClerks")

	time.Sleep(100 * time.Millisecond)

	conf := &kv.Config{}
	err = ts.GetFileJson(kv.KVConfig(ts.job), conf)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Job stats %v", conf)

	err = ts.kvf.Stop()
	assert.Nil(t, err)

	ts.Shutdown()

	return int(conf.Ncoord), int(conf.Nretry), cr
}

func TestKVOK0(t *testing.T) {
	n, r, _ := concurN(t, 0, nil, kv.KVD_NO_REPL)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r)
}

func TestKVOK1(t *testing.T) {
	n, r, cr := concurN(t, 1, nil, kv.KVD_NO_REPL)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r)
	assert.Equal(t, int64(0), cr.Nretry)
}

func TestKVOKN(t *testing.T) {
	n, r, cr := concurN(t, NCLERK, nil, kv.KVD_NO_REPL)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r)
	assert.Equal(t, int64(0), cr.Nretry)
}

func TestClerkPartition1(t *testing.T) {
	n, r, cr := concurN(t, 1, crash.NewTeventMapOne(EvP), kv.KVD_NO_REPL)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r)
	assert.True(t, cr.Nretry > int64(0))
}

func TestCrashBal0(t *testing.T) {
	n, r, cr := concurN(t, 0, balancerEv, kv.KVD_NO_REPL)
	assert.True(t, n > 1)
	assert.Equal(t, 0, r)
	assert.Equal(t, int64(0), cr.Nretry)
}

func TestCrashBal1(t *testing.T) {
	n, r, cr := concurN(t, 1, balancerEv, kv.KVD_NO_REPL)
	assert.True(t, n > 1)
	assert.Equal(t, 0, r)
	assert.Equal(t, int64(0), cr.Nretry)
}

func TestCrashBalN(t *testing.T) {
	n, r, cr := concurN(t, NCLERK, balancerEv, kv.KVD_NO_REPL)
	assert.True(t, n > 1)
	assert.Equal(t, 0, r)
	assert.Equal(t, int64(0), cr.Nretry)
}

func TestCrashMov0(t *testing.T) {
	n, r, _ := concurN(t, 0, moverEv, kv.KVD_NO_REPL)
	assert.Equal(t, 1, n)
	assert.True(t, r > 0)
}

func TestCrashMov1(t *testing.T) {
	n, r, _ := concurN(t, 1, moverEv, kv.KVD_NO_REPL)
	assert.Equal(t, 1, n)
	assert.True(t, r > 0)
}

func TestCrashMovN(t *testing.T) {
	n, r, _ := concurN(t, NCLERK, moverEv, kv.KVD_NO_REPL)
	assert.Equal(t, 1, n)
	assert.True(t, r > 0)
}

func TestCrashAll0(t *testing.T) {
	n, r, _ := concurN(t, 0, bothEv, kv.KVD_NO_REPL)
	assert.True(t, n > 1)
	assert.True(t, r > 0)
}

func TestCrashAll1(t *testing.T) {
	n, r, _ := concurN(t, 1, bothEv, kv.KVD_NO_REPL)
	assert.True(t, n > 1)
	assert.True(t, r > 0)
}

func TestCrashAllN(t *testing.T) {
	n, r, _ := concurN(t, NCLERK, bothEv, kv.KVD_NO_REPL)
	assert.True(t, n > 1)
	assert.True(t, r > 0)
}

func TestCrashAllPartition1(t *testing.T) {
	bothEv.Insert(EvP)
	n, r, cr := concurN(t, 1, bothEv, kv.KVD_NO_REPL)
	assert.True(t, n > 1)
	assert.True(t, r > 0)
	assert.True(t, cr.Nretry > int64(0))
}

func TestReplOK0(t *testing.T) {
	n, r, _ := concurN(t, 0, nil, kv.KVD_REPL_LEVEL)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r)
}

func TestReplOK1(t *testing.T) {
	n, r, _ := concurN(t, 1, nil, kv.KVD_REPL_LEVEL)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r)
}

func TestReplOKN(t *testing.T) {
	n, r, _ := concurN(t, NCLERK, nil, kv.KVD_REPL_LEVEL)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, r)
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
