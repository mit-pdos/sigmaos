package sorteddir

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
)

var NAMES = []string{"a", "b.txt", "gutenberg", "ls.PDF", "wiki"}

func TestBasic(t *testing.T) {
	sd := NewSortedDir[string, *bool]()
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
