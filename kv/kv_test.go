package kv

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	kc, err := MakeClerk()
	assert.Nil(t, err, "MakeClerk")

	for i := 0; i < 10; i++ {
		err := kc.Put(strconv.Itoa(i), strconv.Itoa(i))
		assert.Nil(t, err, "Put")
	}
	for i := 0; i < 10; i++ {
		v, err := kc.Get(strconv.Itoa(i))
		assert.Nil(t, err, "Get")
		assert.Equal(t, v, strconv.Itoa(i), "Get")
	}
}
