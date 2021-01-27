package kv

// XXX assumes a running named, schedd, sharder

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func spawnKv(t *testing.T, kc *KvClerk) {
	err := kc.WriteFile(SHARDER+"/dev", []byte("Add"))
	assert.Nil(t, err, "spawnKv")
}

func delKv(t *testing.T, kc *KvClerk) {
	err := kc.WriteFile(SHARDER+"/dev", []byte("Del"))
	assert.Nil(t, err, "delKv")
}

func TestBasic(t *testing.T) {
	kc, err := MakeClerk()
	assert.Nil(t, err, "MakeClerk")

	for i := 0; i < 100; i++ {
		err := kc.Put(strconv.Itoa(i), strconv.Itoa(i))
		assert.Nil(t, err, "Put")
	}

	for r := 0; r < 1; r++ {
		spawnKv(t, kc)

		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 100; i++ {
			k := strconv.Itoa(i)
			v, err := kc.Get(k)
			assert.Nil(t, err, "Get "+k)
			assert.Equal(t, v, k, "Get")
		}

	}

	for r := 1; r > 0; r-- {
		delKv(t, kc)

		time.Sleep(100 * time.Millisecond)

		for i := 0; i < 100; i++ {
			k := strconv.Itoa(i)
			v, err := kc.Get(k)
			assert.Nil(t, err, "Get "+k)
			assert.Equal(t, v, k, "Get")
		}
	}
}
