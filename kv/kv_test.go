package kv

// XXX assumes a running named, schedd, sharder

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func spawnKv(t *testing.T, kc *KvClerk) {
	for {
		err := kc.WriteFile(SHARDER+"/dev", []byte("Add"))
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func delKv(t *testing.T, kc *KvClerk) {
	for {
		err := kc.WriteFile(SHARDER+"/dev", []byte("Del"))
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func getKeys(t *testing.T, kc *KvClerk) {
	for i := 0; i < 100; i++ {
		k := strconv.Itoa(i)
		v, err := kc.Get(k)
		assert.Nil(t, err, "Get "+k)
		assert.Equal(t, v, k, "Get")
	}
}

// func TestBasic(t *testing.T) {
// 	kc, err := MakeClerk()
// 	assert.Nil(t, err, "MakeClerk")

// 	for i := 0; i < 100; i++ {
// 		err := kc.Put(strconv.Itoa(i), strconv.Itoa(i))
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

func clerk(t *testing.T, kc *KvClerk, done *bool) {
	for !*done {
		getKeys(t, kc)
	}
}

func TestConcur(t *testing.T) {
	done := false

	kc, err := MakeClerk()
	assert.Nil(t, err, "MakeClerk")

	for i := 0; i < 100; i++ {
		err := kc.Put(strconv.Itoa(i), strconv.Itoa(i))
		assert.Nil(t, err, "Put")
	}

	go clerk(t, kc, &done)

	for r := 0; r < NSHARD-1; r++ {
		spawnKv(t, kc)
		time.Sleep(1000 * time.Millisecond)
	}

	for r := NSHARD - 1; r > 0; r-- {
		delKv(t, kc)
		time.Sleep(1000 * time.Millisecond)
	}

	done = true
}
