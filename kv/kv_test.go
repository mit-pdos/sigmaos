package kv

// XXX assumes a running named, schedd, sharder and kvd

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
)

func TestBasic(t *testing.T) {
	kc, err := MakeClerk()
	assert.Nil(t, err, "MakeClerk")

	for i := 0; i < 10; i++ {
		err := kc.Put(strconv.Itoa(i), strconv.Itoa(i))
		assert.Nil(t, err, "Put")
	}

	a := fslib.Attr{}
	a.Pid = strconv.Itoa(rand.Intn(100000))
	a.Program = "./bin/kvd"
	a.Args = []string{"1"}
	a.PairDep = nil
	a.ExitDep = nil

	err = kc.Spawn(&a)
	assert.Nil(t, err, "Spawn")

	time.Sleep(1000 * time.Millisecond)

	for i := 0; i < 10; i++ {
		k := strconv.Itoa(i)
		v, err := kc.Get(k)
		assert.Nil(t, err, "Get "+k)
		assert.Equal(t, v, k, "Get")
	}
}
