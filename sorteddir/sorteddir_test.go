package sorteddir

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var NAMES = []string{"a", "b.txt", "gutenberg", "ls.PDF", "wiki"}

func TestBasic(t *testing.T) {
	sd := MkSortedDir()
	for _, n := range NAMES {
		sd.Insert(n, nil)
	}
	i := 0
	sd.Iter(func(n string, e interface{}) bool {
		assert.Equal(t, NAMES[i], n)
		i += 1
		return true
	})
	sd.Delete("a")
	assert.Equal(t, len(NAMES)-1, len(sd.dents))
	assert.Equal(t, len(NAMES)-1, len(sd.sorted))
	i = 1
	sd.Iter(func(n string, e interface{}) bool {
		assert.Equal(t, NAMES[i], n)
		i += 1
		return true
	})
}
