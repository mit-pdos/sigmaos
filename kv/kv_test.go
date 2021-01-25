package kv

// XXX assumes a running named, schedd, sharder

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	kc, err := MakeClerk()
	assert.Nil(t, err, "MakeClerk")

	for i := 0; i < 10; i++ {
		err := kc.Put(strconv.Itoa(i), strconv.Itoa(i))
		assert.Nil(t, err, "Put")
	}

	err = kc.WriteFile(SHARDER+"/dev", []byte("Add"))
	assert.Nil(t, err, "Spawn")

	time.Sleep(1000 * time.Millisecond)

	for i := 0; i < 10; i++ {
		k := strconv.Itoa(i)
		v, err := kc.Get(k)
		assert.Nil(t, err, "Get "+k)
		assert.Equal(t, v, k, "Get")
	}
}
