package sortedmapv1

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/btree"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/util/rand"
)

var NAMES = []string{"a", "b.txt", "gutenberg", "ls.PDF", "wiki"}

func TestBasic(t *testing.T) {
	sd := NewSortedMap[string, *bool]()
	for i, _ := range NAMES {
		j := len(NAMES) - (i + 1)
		sd.Insert(NAMES[j], new(bool))
	}
	db.DPrintf(db.TEST, "sd %v\n", sd)
	i := 0
	sd.Iter(func(n string, b *bool) bool {
		assert.Equal(t, NAMES[i], n)
		i += 1
		return true
	})
	sd.Delete("a")
	assert.Equal(t, len(NAMES)-1, sd.Len())
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

	sd.Insert("1", new(bool))
	i, ok = sd.RoundRobin()
	assert.True(t, ok)
	j, ok := sd.RoundRobin()
	assert.True(t, ok)
	assert.True(t, i == j)
	sd.Delete(i)

	for i, _ := range NAMES {
		j := len(NAMES) - (i + 1)
		sd.Insert(NAMES[j], new(bool))
	}

	i, ok = sd.RoundRobin()
	assert.True(t, ok)
	assert.Equal(t, "a", i)

	db.DPrintf(db.TEST, "sd %v\n", sd)
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

func perm(n int) (ns []int) {
	for i := 0; i < n; i++ {
		r := rand.Int64(int64(n))
		ns = append(ns, int(r))
	}
	return ns
}

func TestMany(t *testing.T) {
	ns := []int{10, 100, 1000, 10_000, 100_000}
	// ns := []int{10_000}
	for _, n := range ns {
		rs := perm(n)
		sd := NewSortedMap[string, *bool]()
		s := time.Now()
		for i := 0; i < n; i++ {
			sd.Insert("ff"+strconv.Itoa(rs[i]), new(bool))
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d ops %v us/op %v", n, t, float64(t.Microseconds())/float64(n))
	}
}

func TestBtree(t *testing.T) {
	ns := []int{10, 100, 1000, 10_000, 100_000}
	// ns := []int{10_000}
	for _, n := range ns {
		rs := perm(n)
		tr := btree.NewOrderedG[string](32)
		s := time.Now()
		for i := 0; i < n; i++ {
			tr.ReplaceOrInsert("ff" + strconv.Itoa(rs[i]))
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d ops %v us/op %v", n, t, float64(t.Microseconds())/float64(n))
	}
}
