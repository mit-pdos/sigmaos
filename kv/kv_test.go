package kv

import (
	"log"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/test"
)

const (
	NBALANCER = 3
	NCLERK    = 10

	CRASHBALANCER = 1000
	CRASHHELPER   = "200"
)

func checkKvs(t *testing.T, kvs *KvSet, n int) {
	for _, v := range kvs.set {
		if v != n {
			assert.Equal(t, v, n+1, "checkKvs")
		}
	}
}

func TestBalance(t *testing.T) {
	conf := &Config{}
	for i := 0; i < NSHARD; i++ {
		conf.Shards = append(conf.Shards, "")
	}
	for k := 0; k < NKV; k++ {
		shards := AddKv(conf, strconv.Itoa(k))
		conf.Shards = shards
		kvs := makeKvs(conf.Shards)
		//log.Printf("balance %v %v\n", shards, kvs)
		checkKvs(t, kvs, NSHARD/(k+1))
	}
	for k := NKV - 1; k > 0; k-- {
		shards := DelKv(conf, strconv.Itoa(k))
		conf.Shards = shards
		kvs := makeKvs(conf.Shards)
		//log.Printf("balance %v %v\n", shards, kvs)
		checkKvs(t, kvs, NSHARD/k)
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
	clrk    *KvClerk
	mfsgrps []*groupmgr.GroupMgr
	gmbal   *groupmgr.GroupMgr
	clrks   []proc.Tpid
}

func makeTstate(t *testing.T, auto string, nclerk int, crash int, crashhelper string) (*Tstate, *KvClerk) {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.gmbal = groupmgr.Start(ts.System.FsLib, ts.System.ProcClnt, NBALANCER, "bin/user/balancer", []string{crashhelper, auto}, NBALANCER, crash)

	clrk := ts.setup(nclerk)
	return ts, clrk
}

func (ts *Tstate) setup(nclerk int) *KvClerk {
	// Create first shard group
	gn := group.GRP + "0"
	grp := SpawnGrp(ts.FsLib, ts.ProcClnt, gn)
	err := ts.balancerOp("add", gn)
	assert.Nil(ts.T, err, "BalancerOp")
	ts.mfsgrps = append(ts.mfsgrps, grp)

	// Create keys
	clrk, err := MakeClerk("kv_test", fslib.Named())
	assert.Nil(ts.T, err, "MakeClerk")
	for i := uint64(0); i < NKEYS; i++ {
		err := clrk.Put(MkKey(i), []byte{})
		assert.Nil(ts.T, err, "Put")
	}
	return clrk
}

func (ts *Tstate) done() {
	ts.gmbal.Stop()
	ts.stopMemfsgrps()
	ts.Shutdown()
}

func (ts *Tstate) stopFS(fs proc.Tpid) {
	err := ts.Evict(fs)
	assert.Nil(ts.T, err, "stopFS")
	ts.WaitExit(fs)
}

func (ts *Tstate) stopMemfsgrps() {
	for _, gm := range ts.mfsgrps {
		gm.Stop()
	}
}

func (ts *Tstate) stopClerks() {
	log.Printf("clerks to evict %v\n", len(ts.clrks))
	for _, ck := range ts.clrks {
		err := ts.Evict(ck)
		assert.Nil(ts.T, err, "stopClerks")
		status, err := ts.WaitExit(ck)
		assert.Nil(ts.T, err, "WaitExit")
		assert.True(ts.T, status.IsStatusOK(), "Exit status: %v", status)
	}
}

func (ts *Tstate) startClerk() proc.Tpid {
	p := proc.MakeProc("bin/user/kv-clerk", []string{""})
	ts.Spawn(p)
	err := ts.WaitStart(p.Pid)
	assert.Nil(ts.T, err, "WaitStart")
	return p.Pid
}

func (ts *Tstate) balancerOp(opcode, mfs string) error {
	for true {
		err := BalancerOp(ts.FsLib, opcode, mfs)
		if err == nil {
			return nil
		}
		if np.IsErrUnavailable(err) || np.IsErrRetry(err) {
			log.Printf("balancer op wait err %v\n", err)
			time.Sleep(100 * time.Millisecond)
		} else {
			log.Printf("balancer op err %v\n", err)
			return err
		}
	}
	return nil
}

func TestGetPutSet(t *testing.T) {
	ts, clrk := makeTstate(t, "manual", 1, 0, "0")

	_, err := clrk.Get(MkKey(NKEYS+1), 0)
	assert.NotEqual(ts.T, err, nil, "Get")

	err = clrk.Set(MkKey(NKEYS+1), []byte(MkKey(NKEYS+1)), 0)
	assert.NotEqual(ts.T, err, nil, "Set")

	err = clrk.Set(MkKey(0), []byte(MkKey(0)), 0)
	assert.Nil(ts.T, err, "Set")

	for i := uint64(0); i < NKEYS; i++ {
		key := MkKey(i)
		_, err := clrk.Get(key, 0)
		assert.Nil(ts.T, err, "Get "+key.String())
	}

	ts.done()
}

func concurN(t *testing.T, nclerk int, crash int, crashhelper string) {
	const TIME = 100 // 500

	ts, _ := makeTstate(t, "manual", nclerk, crash, crashhelper)

	for i := 0; i < nclerk; i++ {
		pid := ts.startClerk()
		ts.clrks = append(ts.clrks, pid)
	}

	for s := 0; s < NKV; s++ {
		grp := group.GRP + strconv.Itoa(s+1)
		gm := SpawnGrp(ts.FsLib, ts.ProcClnt, grp)
		ts.mfsgrps = append(ts.mfsgrps, gm)
		err := ts.balancerOp("add", grp)
		assert.Nil(ts.T, err, "BalancerOp")
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	for s := 0; s < NKV; s++ {
		grp := group.GRP + strconv.Itoa(len(ts.mfsgrps)-1)
		err := ts.balancerOp("del", grp)
		assert.Nil(ts.T, err, "BalancerOp")
		ts.mfsgrps[len(ts.mfsgrps)-1].Stop()
		ts.mfsgrps = ts.mfsgrps[0 : len(ts.mfsgrps)-1]
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	ts.stopClerks()

	log.Printf("done waiting for clerks\n")

	time.Sleep(100 * time.Millisecond)

	ts.gmbal.Stop()

	log.Printf("done waiting for balancer\n")

	ts.mfsgrps[0].Stop()

	log.Printf("done waiting for kv 0\n")

	ts.Shutdown()

	log.Printf("done shutdown kv 0\n")
}

func TestConcurOK0(t *testing.T) {
	concurN(t, 0, 0, "0")
}

func TestConcurOK1(t *testing.T) {
	concurN(t, 1, 0, "0")
}

func TestConcurOKN(t *testing.T) {
	concurN(t, NCLERK, 0, "0")
}

func TestConcurFailBal0(t *testing.T) {
	concurN(t, 0, CRASHBALANCER, "0")
}

func TestConcurFailBal1(t *testing.T) {
	concurN(t, 1, CRASHBALANCER, "0")
}

func TestConcurFailBalN(t *testing.T) {
	concurN(t, NCLERK, CRASHBALANCER, "0")
}

func TestConcurFailAll0(t *testing.T) {
	concurN(t, 0, CRASHBALANCER, CRASHHELPER)
}

func TestConcurFailAll1(t *testing.T) {
	concurN(t, 1, CRASHBALANCER, CRASHHELPER)
}

func TestConcurFailAllN(t *testing.T) {
	concurN(t, NCLERK, CRASHBALANCER, CRASHHELPER)
}

// func TestAuto(t *testing.T) {
// 	// runtime.GOMAXPROCS(2) // XXX for KV
// 	nclerk := NCLERK
// 	ts := makeTstate(t, "auto", nclerk, 0)

// 	ch := make(chan bool)
// 	for i := 0; i < nclerk; i++ {
// 		go ts.clerk(i, ch)
// 	}

// 	time.Sleep(30 * time.Second)

// 	log.Printf("Wait for clerks\n")

// 	for i := 0; i < nclerk; i++ {
// 		ch <- true
// 	}

// 	ts.done()
// }
