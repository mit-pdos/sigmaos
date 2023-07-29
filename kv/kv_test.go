package kv_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"google.golang.org/protobuf/proto"

	// "google.golang.org/protobuf/reflect/protoreflect"

	cproto "sigmaos/cache/proto"

	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/kv"
	kproto "sigmaos/kv/proto"
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

func decode(t *testing.T, b []byte, m proto.Message) {
	typ := reflect.TypeOf(m)
	rdr := bytes.NewReader(b)
	log.Printf("b = %d\n", len(b))
	for {
		var l uint32
		if err := binary.Read(rdr, binary.LittleEndian, &l); err != nil {
			if err == io.EOF {
				break
			}
			assert.Nil(t, err)
		}
		log.Printf("len %d\n", l)
		b := make([]byte, int(l))
		if _, err := io.ReadFull(rdr, b); err != nil && !(err == io.EOF && l == 0) {
			assert.Nil(t, err)
		}
		val := reflect.New(typ.Elem()).Interface().(proto.Message)
		log.Printf("type %T %v\n", val, typ)
		if err := proto.Unmarshal(b, val); err != nil {
			assert.Nil(t, err)
		}
		log.Printf("val %v\n", val)
		// vals = append(vals, val)
	}
}

func TestProtoArray(t *testing.T) {
	b, err := proto.Marshal(&kproto.KVTestVal{Key: "xxx"})
	assert.Nil(t, err)
	var buf bytes.Buffer
	l := uint32(len(b))
	wr := bufio.NewWriter(&buf)
	for i := 0; i < 2; i++ {
		if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
			assert.Nil(t, err)
		}
		if err := binary.Write(wr, binary.LittleEndian, b); err != nil {
			assert.Nil(t, err)
		}
	}
	wr.Flush()
	decode(t, buf.Bytes(), &kproto.KVTestVal{})
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
	kvf *kv.KVFleet
	cm  *kv.ClerkMgr
}

func makeTstate(t *testing.T, auto string, crashbal, repl, ncrash int, crashhelper string) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	job := rand.String(4)

	kvf, err := kv.MakeKvdFleet(ts.SigmaClnt, job, 1, repl, 0, crashhelper, auto)
	assert.Nil(t, err)
	ts.kvf = kvf
	ts.cm, err = kv.MkClerkMgr(ts.SigmaClnt, job, 0)
	assert.Nil(t, err)
	err = ts.kvf.Start()
	assert.Nil(t, err)
	err = ts.cm.StartCmClerk()
	assert.Nil(t, err)
	err = ts.cm.InitKeys(kv.NKEYS)
	assert.Nil(t, err)
	return ts
}

func (ts *Tstate) done() {
	ts.cm.StopClerks()
	ts.kvf.Stop()
	ts.Shutdown()
}

func TestMiss(t *testing.T) {
	ts := makeTstate(t, "manual", 0, kv.KVD_NO_REPL, 0, "0")
	err := ts.cm.Get(kv.MkKey(kv.NKEYS+1), &cproto.CacheString{})
	assert.Equal(t, cachesrv.ErrMiss, err)
	ts.done()
}

func TestGetPut(t *testing.T) {
	ts := makeTstate(t, "manual", 0, kv.KVD_NO_REPL, 0, "0")

	err := ts.cm.Get(kv.MkKey(kv.NKEYS+1), &cproto.CacheString{})
	assert.NotNil(ts.T, err, "Get")

	err = ts.cm.Put(kv.MkKey(kv.NKEYS+1), &cproto.CacheString{Val: ""})
	assert.Nil(ts.T, err, "Put")

	err = ts.cm.Put(kv.MkKey(0), &cproto.CacheString{Val: ""})
	assert.Nil(ts.T, err, "Put")

	for i := uint64(0); i < kv.NKEYS; i++ {
		key := kv.MkKey(i)
		err := ts.cm.Get(key, &cproto.CacheString{})
		assert.Nil(ts.T, err, "Get "+key)
	}

	ts.cm.StopClerks()
	ts.done()
}

func concurN(t *testing.T, nclerk, crashbal, repl, ncrash int, crashhelper string) {
	const TIME = 100

	ts := makeTstate(t, "manual", crashbal, repl, ncrash, crashhelper)

	err := ts.cm.StartClerks("", nclerk)
	assert.Nil(ts.T, err, "Error StartClerk: %v", err)

	db.DPrintf(db.TEST, "Done StartClerks")

	for i := 0; i < kv.NKV; i++ {
		err := ts.kvf.AddKVDGroup()
		assert.Nil(ts.T, err, "AddKVDGroup")
		// allow some puts/gets
		time.Sleep(TIME * time.Millisecond)
	}

	db.DPrintf(db.TEST, "Done adds")

	for i := 0; i < kv.NKV; i++ {
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

func XTestConcurReplOK0(t *testing.T) {
	concurN(t, 0, 0, kv.KVD_REPL_LEVEL, 0, "0")
}

func XTestConcurReplOK1(t *testing.T) {
	concurN(t, 1, 0, kv.KVD_REPL_LEVEL, 0, "0")
}

//
// Fix: Repl tests fail now because lack of shard replication.
//

func XTestConcurReplOKN(t *testing.T) {
	concurN(t, NCLERK, 0, kv.KVD_REPL_LEVEL, 0, "0")
}

func XTestConcurReplFail0(t *testing.T) {
	concurN(t, 0, 0, kv.KVD_REPL_LEVEL, 1, "0")
}

func XTestConcurReplFail1(t *testing.T) {
	concurN(t, 1, 0, kv.KVD_REPL_LEVEL, 1, "0")
}

func XTestConcurReplFailN(t *testing.T) {
	concurN(t, NCLERK, 0, kv.KVD_REPL_LEVEL, 1, "0")
}

func TestAuto(t *testing.T) {
	// runtime.GOMAXPROCS(2) // XXX for KV

	ts := makeTstate(t, "manual", 0, kv.KVD_NO_REPL, 0, "0")

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
