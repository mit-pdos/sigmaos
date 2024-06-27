package sortedmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
)

var NAMES = []string{"a", "b.txt", "gutenberg", "ls.PDF", "wiki"}

func TestBasic(t *testing.T) {
	sd := NewSortedMap[string, *bool]()
	for i, _ := range NAMES {
		j := len(NAMES) - (i + 1)
		sd.Insert(NAMES[j], new(bool))
	}
	db.DPrintf(db.TEST, "sd %v\n", sd.sorted)
	i := 0
	sd.Iter(func(n string, b *bool) bool {
		assert.Equal(t, NAMES[i], n)
		i += 1
		return true
	})
	sd.Delete("a")
	assert.Equal(t, len(NAMES)-1, len(sd.dents))
	assert.Equal(t, len(NAMES)-1, len(sd.sorted))
	i = 1
	sd.Iter(func(n string, b *bool) bool {
		assert.Equal(t, NAMES[i], n)
		i += 1
		return true
	})
}

func TestPresent(t *testing.T) {
	k := "k"
	sd := NewSortedMap[string, *bool]()
	ok := sd.InsertKey(k)
	assert.True(t, ok)
	ok, _, vok := sd.LookupKeyVal(k)
	assert.True(t, ok)
	assert.False(t, vok)
	b := new(bool)
	*b = true
	sd.Insert(k, b)
	ok, v, vok := sd.LookupKeyVal(k)
	assert.True(t, ok)
	assert.True(t, vok)
	assert.True(t, *v)
}

func TestRR(t *testing.T) {
	sd := NewSortedMap[string, *bool]()

	i, ok := sd.RoundRobin()
	assert.False(t, ok)

	for i, _ := range NAMES {
		j := len(NAMES) - (i + 1)
		sd.Insert(NAMES[j], new(bool))
	}

	i, ok = sd.RoundRobin()
	assert.True(t, ok)
	assert.Equal(t, "a", i)

	db.DPrintf(db.TEST, "sd %v\n", sd.sorted)
	ok = sd.Delete("a")
	assert.True(t, ok)

	i, ok = sd.RoundRobin()
	assert.True(t, ok)
	assert.Equal(t, "b.txt", i)

	ok = sd.Insert("a", new(bool))
	assert.True(t, ok)

	i, ok = sd.RoundRobin()
	assert.True(t, ok)
	assert.Equal(t, "gutenberg", i)
}
