package kv

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
)

const BIN = "../bin"

type Tstate struct {
	t   *testing.T
	s   *fslib.System
	fsl *fslib.FsLib
	*KvClerk
	ch chan bool
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.ch = make(chan bool)

	s, err := fslib.Boot(BIN)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	ts.fsl = fslib.MakeFsLib("tester")

	err = ts.fsl.Mkdir("name/kv", 0777)
	if err != nil {
		t.Fatalf("Mkdir %v\n", err)
	}

	ts.spawnSharder()

	time.Sleep(100 * time.Millisecond)

	ts.spawnKv()

	time.Sleep(100 * time.Millisecond)

	ts.spawnSharder()

	time.Sleep(1000 * time.Millisecond)

	kc, err := MakeClerk()
	if err != nil {
		t.Fatalf("Make clerk %v\n", err)
	}
	ts.KvClerk = kc

	return ts
}

func (ts *Tstate) spawnKv() {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = BIN + "/kvd"
	a.Args = []string{}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
}

func (ts *Tstate) spawnSharder() {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = BIN + "/sharderd"
	a.Args = []string{BIN}
	a.PairDep = nil
	a.ExitDep = nil
	ts.fsl.Spawn(&a)
}

func (ts *Tstate) delKv() {
	for {
		err := ts.WriteFile(SHARDER+"/dev", []byte("Del"))
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ts *Tstate) getKeys() {
	for i := 0; i < 100; i++ {
		k := strconv.Itoa(i)
		v, err := ts.Get(k)
		assert.Nil(ts.t, err, "Get "+k)
		assert.Equal(ts.t, v, k, "Get")
	}
}

// func TestBasic(t *testing.T) {
// 	kc, err := MakeClerk()
// 	assert.Nil(t, err, "MakeClerk")

// 	for i := 0; i < 100; i++ {
// 		err := ts.Put(strconv.Itoa(i), strconv.Itoa(i))
// 		assert.Nil(t, err, "Put")
// 	}

// 	for r := 0; r < NSHARD-1; r++ {
// 		spawnKv(t, kc)
// 		time.Sleep(100 * time.Millisecond)
// 		getKeys(t, kc)
// 	}

// 	for r := NSHARD - 1; r > 0; r-- {
// 		delKv(t, kc)
// 		time.Sleep(100 * time.Millisecond)
// 		getKeys(t, kc)
// 	}
// }

func (ts *Tstate) clerk() {
	for {
		select {
		case <-ts.ch:
			break
		default:
			ts.getKeys()
		}
	}
}

func TestConcur(t *testing.T) {
	ts := makeTstate(t)

	for true {
		time.Sleep(1000 * time.Millisecond)
	}

	for i := 0; i < 100; i++ {
		err := ts.Put(strconv.Itoa(i), strconv.Itoa(i))
		assert.Nil(t, err, "Put")
	}

	go ts.clerk()

	for r := 0; r < NSHARD-1; r++ {
		ts.spawnKv()
		time.Sleep(1000 * time.Millisecond)
	}

	for r := NSHARD - 1; r > 0; r-- {
		ts.delKv()
		time.Sleep(1000 * time.Millisecond)
	}

	ts.ch <- true
}
