package kv_test

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/group"
	"sigmaos/groupmgr"
	"sigmaos/kv"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/test"
)

const (
	NCLERK = 4

	CRASHBALANCER = 1000
	CRASHMOVER    = "200"
)

func checkKvs(t *testing.T, kvs *kv.KvSet, n int) {
	for _, v := range kvs.Set {
		if v != n {
			assert.Equal(t, v, n+1, "checkKvs")
		}
	}
}

func TestBalance(t *testing.T) {
	conf := &kv.Config{}
	for i := 0; i < kv.NSHARD; i++ {
		conf.Shards = append(conf.Shards, "")
	}
	for k := 0; k < kv.NKV; k++ {
		shards := kv.AddKv(conf, strconv.Itoa(k))
		conf.Shards = shards
		kvs := kv.MakeKvs(conf.Shards)
		//db.DPrintf(db.ALWAYS, "balance %v %v\n", shards, kvs)
		checkKvs(t, kvs, kv.NSHARD/(k+1))
	}
	for k := kv.NKV - 1; k > 0; k-- {
		shards := kv.DelKv(conf, strconv.Itoa(k))
		conf.Shards = shards
		kvs := kv.MakeKvs(conf.Shards)
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
	clrk    *kv.KvClerk
	mfsgrps []*groupmgr.GroupMgr
	gmbal   *groupmgr.GroupMgr
	clrks   []proc.Tpid
	job     string
}

func makeTstate(t *testing.T, auto string, crashbal, repl, ncrash int, crashhelper string) (*Tstate, *kv.KvClerk) {
	ts := &Tstate{}
	ts.job = rand.String(16)
	ts.Tstate = test.MakeTstateAll(t)
	ts.gmbal = kv.StartBalancers(ts.FsLib, ts.ProcClnt, ts.job, kv.NBALANCER, crashbal, 0, crashhelper, auto)
	clrk := ts.setup(repl, ncrash)
	return ts, clrk
}

func (ts *Tstate) setup(repl, ncrash int) *kv.KvClerk {
	// Create first shard group
	gn := group.GRP + "0"
	grp := kv.SpawnGrp(ts.FsLib, ts.ProcClnt, ts.job, gn, 0, repl, ncrash)
	err := kv.BalancerOpRetry(ts.FsLib, ts.job, "add", gn)
	assert.Nil(ts.T, err, "BalancerOp")
	ts.mfsgrps = append(ts.mfsgrps, grp)

	// Create keys
	clrk, err := kv.InitKeys(ts.FsLib, ts.ProcClnt, ts.job, kv.NKEYS)
	assert.Nil(ts.T, err, "InitKeys: %v", err)
	return clrk
}

func (ts *Tstate) done() {
	ts.gmbal.Stop()
	ts.stopMemfsgrps()
	ts.Shutdown()
}

func (ts *Tstate) stopMemfsgrps() {
	for _, gm := range ts.mfsgrps {
		gm.Stop()
	}
}

func (ts *Tstate) stopClerks() {
	db.DPrintf(db.ALWAYS, "clerks to evict %v\n", len(ts.clrks))
	for _, ck := range ts.clrks {
		status, err := kv.StopClerk(ts.ProcClnt, ck)
		assert.Nil(ts.T, err, "StopClerk: %v", err)
		assert.True(ts.T, status.IsStatusEvicted(), "Exit status: %v", status)
	}
}

func TestGetPutSet(t *testing.T) {
	ts, clrk := makeTstate(t, "manual", 0, kv.KVD_NO_REPL, 0, "0")

	_, err := clrk.Get(kv.MkKey(kv.NKEYS+1), 0)
	assert.NotEqual(ts.T, err, nil, "Get")

	err = clrk.Set(kv.MkKey(kv.NKEYS+1), []byte(kv.MkKey(kv.NKEYS+1)), 0)
	assert.NotEqual(ts.T, err, nil, "Set")

	err = clrk.Set(kv.MkKey(0), []byte(kv.MkKey(0)), 0)
	assert.Nil(ts.T, err, "Set")

	for i := uint64(0); i < kv.NKEYS; i++ {
		key := kv.MkKey(i)
		_, err := clrk.Get(key, 0)
		assert.Nil(ts.T, err, "Get "+key.String())
	}

	ts.done()
}

func concurN(t *testing.T, nclerk, crashbal, repl, ncrash int, crashhelper string) {
	const TIME = 100

	ts, _ := makeTstate(t, "manual", crashbal, repl, ncrash, crashhelper)

	for i := 0; i < nclerk; i++ {
		pid, err := kv.StartClerk(ts.ProcClnt, ts.job, nil, 0)
		assert.Nil(ts.T, err, "Error StartClerk: %v", err)
		ts.clrks = append(ts.clrks, pid)
	}

	db.DPrintf("TEST", "Done startClerks")

	for s := 0; s < kv.NKV; s++ {
		grp := group.GRP + strconv.Itoa(s+1)
		gm := kv.SpawnGrp(ts.FsLib, ts.ProcClnt, ts.job, grp, 0, repl, ncrash)
		ts.mfsgrps = append(ts.mfsgrps, gm)
		err := kv.BalancerOpRetry(ts.FsLib, ts.job, "add", grp)
		assert.Nil(ts.T, err, "BalancerOp")
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	db.DPrintf("TEST", "Done adds")

	for s := 0; s < kv.NKV; s++ {
		grp := group.GRP + strconv.Itoa(len(ts.mfsgrps)-1)
		err := kv.BalancerOpRetry(ts.FsLib, ts.job, "del", grp)
		assert.Nil(ts.T, err, "BalancerOp")
		ts.mfsgrps[len(ts.mfsgrps)-1].Stop()
		ts.mfsgrps = ts.mfsgrps[0 : len(ts.mfsgrps)-1]
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	db.DPrintf("TEST", "Done dels")

	ts.stopClerks()

	db.DPrintf("TEST", "Done stopClerks")

	time.Sleep(100 * time.Millisecond)

	ts.gmbal.Stop()

	ts.mfsgrps[0].Stop()

	ts.Shutdown()
}

func TestConcurOK0(t *testing.T) {
	concurN(t, 0, 0, kv.KVD_NO_REPL, 0, "0")
}

func TestConcurOK1(t *testing.T) {
	concurN(t, 1, 0, kv.KVD_NO_REPL, 0, "0")
}

func TestConcurOKN(t *testing.T) {
	concurN(t, NCLERK, 0, kv.KVD_NO_REPL, 0, "0")
}

func TestConcurFailBal0(t *testing.T) {
	concurN(t, 0, CRASHBALANCER, kv.KVD_NO_REPL, 0, "0")
}

func TestConcurFailBal1(t *testing.T) {
	concurN(t, 1, CRASHBALANCER, kv.KVD_NO_REPL, 0, "0")
}

func TestConcurFailBalN(t *testing.T) {
	concurN(t, NCLERK, CRASHBALANCER, kv.KVD_NO_REPL, 0, "0")
}

func TestConcurFailAll0(t *testing.T) {
	concurN(t, 0, CRASHBALANCER, kv.KVD_NO_REPL, 0, CRASHMOVER)
}

func TestConcurFailAll1(t *testing.T) {
	concurN(t, 1, CRASHBALANCER, kv.KVD_NO_REPL, 0, CRASHMOVER)
}

func TestConcurFailAllN(t *testing.T) {
	concurN(t, NCLERK, CRASHBALANCER, kv.KVD_NO_REPL, 0, CRASHMOVER)
}

func TestConcurReplOK0(t *testing.T) {
	concurN(t, 0, 0, kv.KVD_REPL_LEVEL, 0, "0")
}

func TestConcurReplOK1(t *testing.T) {
	concurN(t, 1, 0, kv.KVD_REPL_LEVEL, 0, "0")
}

func TestConcurReplOKN(t *testing.T) {
	concurN(t, NCLERK, 0, kv.KVD_REPL_LEVEL, 0, "0")
}

func TestConcurReplFail0(t *testing.T) {
	concurN(t, 0, 0, kv.KVD_REPL_LEVEL, 1, "0")
}

func TestConcurReplFail1(t *testing.T) {
	concurN(t, 1, 0, kv.KVD_REPL_LEVEL, 1, "0")
}

func TestConcurReplFailN(t *testing.T) {
	concurN(t, NCLERK, 0, kv.KVD_REPL_LEVEL, 1, "0")
}

func TestAuto(t *testing.T) {
	// runtime.GOMAXPROCS(2) // XXX for KV

	nclerk := NCLERK
	ts, _ := makeTstate(t, "auto", 0, kv.KVD_NO_REPL, 0, "0")

	for i := 0; i < nclerk; i++ {
		pid, err := kv.StartClerk(ts.ProcClnt, ts.job, nil, 0)
		assert.Nil(ts.T, err, "Error StartClerk: %v", err)
		ts.clrks = append(ts.clrks, pid)
	}

	time.Sleep(10 * time.Second)

	db.DPrintf(db.ALWAYS, "Wait for clerks\n")

	ts.stopClerks()

	time.Sleep(100 * time.Millisecond)

	ts.gmbal.Stop()

	ts.mfsgrps[0].Stop()

	ts.Shutdown()
}
