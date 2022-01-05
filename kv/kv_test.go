package kv

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	"ulambda/kernel"
	"ulambda/proc"
)

const (
	NBALANCER = 3
	NCLERK    = 10

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
	t       *testing.T
	clrk    *KvClerk
	mfsgrps []*groupmgr.GroupMgr
	gmbal   *groupmgr.GroupMgr
	clrks   []string
}

func makeTstate(t *testing.T, auto string, nclerk int, crash int) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemAll("kv_test", "..")
	ts.gmbal = groupmgr.Start(ts.System.FsLib, ts.System.ProcClnt, NBALANCER, "bin/user/balancer", []string{auto}, crash)

	ts.setup(nclerk)

	return ts
}

func (ts *Tstate) setup(nclerk int) {
	log.Printf("start kv\n")

	// add 1 kv so that we can put to initialize
	gn := group.GRP + "0"
	grp := SpawnGrp(ts.FsLib, ts.ProcClnt, gn)
	err := ts.balancerOp("add", gn)
	assert.Nil(ts.t, err, "BalancerOp")

	ts.clrk = MakeClerk(fslib.Named())
	if nclerk > 0 {
		for i := uint64(0); i < NKEYS; i++ {
			err := ts.clrk.Put(key(i), key(i))
			assert.Nil(ts.t, err, "Put")
		}
	}
	ts.mfsgrps = append(ts.mfsgrps, grp)
}

func (ts *Tstate) done() {
	ts.gmbal.Stop()
	ts.stopMemfsgrps()
	ts.Shutdown()
}

func (ts *Tstate) stopFS(fs string) {
	err := ts.Evict(fs)
	assert.Nil(ts.t, err, "stopFS")
	ts.WaitExit(fs)
}

func (ts *Tstate) stopMemfsgrps() {
	for _, gm := range ts.mfsgrps {
		gm.Stop()
	}
}

func (ts *Tstate) stopClerks() {
	for _, ck := range ts.clrks {
		err := ts.Evict(ck)
		assert.Nil(ts.t, err, "stopClerks")
		status, err := ts.WaitExit(ck)
		assert.Nil(ts.t, err, "WaitExit")
		assert.Equal(ts.t, "OK", status)
	}
}

func (ts *Tstate) startClerk() string {
	p := proc.MakeProc("bin/user/kv-clerk", []string{""})
	ts.Spawn(p)
	err := ts.WaitStart(p.Pid)
	assert.Nil(ts.t, err, "WaitStart")
	return p.Pid
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
			strings.HasPrefix(err.Error(), "retry") ||
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

	_, err := ts.clrk.Get(key(NKEYS + 1))
	assert.NotEqual(ts.t, err, nil, "Get")

	err = ts.clrk.Set(key(NKEYS+1), key(NKEYS+1))
	assert.NotEqual(ts.t, err, nil, "Set")

	err = ts.clrk.Set(key(0), key(0))
	assert.Nil(ts.t, err, "Set")

	for i := uint64(0); i < NKEYS; i++ {
		v, err := ts.clrk.Get(key(i))
		assert.Nil(ts.t, err, "Get "+key(i))
		assert.Equal(ts.t, key(i), v, "Get")
	}

	ts.done()
}

func concurN(t *testing.T, nclerk int, crash int) {
	const NMORE = 10
	const TIME = 100 // 500

	ts := makeTstate(t, "manual", nclerk, crash)

	for i := 0; i < nclerk; i++ {
		pid := ts.startClerk()
		ts.clrks = append(ts.clrks, pid)
	}

	for s := 0; s < NMORE; s++ {
		grp := group.GRP + strconv.Itoa(s+1)
		gm := SpawnGrp(ts.FsLib, ts.ProcClnt, grp)
		ts.mfsgrps = append(ts.mfsgrps, gm)
		err := ts.balancerOp("add", grp)
		assert.Nil(ts.t, err, "BalancerOp")
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	for s := 0; s < NMORE; s++ {
		grp := group.GRP + strconv.Itoa(len(ts.mfsgrps)-1)
		err := ts.balancerOp("del", grp)
		assert.Nil(ts.t, err, "BalancerOp")
		ts.mfsgrps[len(ts.mfsgrps)-1].Stop()
		ts.mfsgrps = ts.mfsgrps[0 : len(ts.mfsgrps)-1]
		// do some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	ts.stopClerks()

	log.Printf("Done waiting for clerks\n")

	time.Sleep(100 * time.Millisecond)

	ts.gmbal.Stop()

	ts.mfsgrps[0].Stop()

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
