package path

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplit(t *testing.T) {
	s := Split("name/s3/192.168.2.114:43471//b.txt")
	assert.Equal(t, 4, len(s))
	assert.Equal(t, s[3], "b.txt")

	s = Split("name/s3/192.168.2.114:43471//b.txt/")
	assert.Equal(t, 4, len(s))
	assert.Equal(t, s[3], "b.txt")

	s = Split("name/s3////192.168.2.114:43471//b.txt/")
	assert.Equal(t, 4, len(s))
	assert.Equal(t, s[3], "b.txt")
}

func TestIsParent(t *testing.T) {
	assert.True(t, Path{"a"}.IsParent(Path{}))
	assert.False(t, Path{"b"}.IsParent(Path{"a"}))
	assert.True(t, Path{"a", "b"}.IsParent(Path{"a"}))
	assert.False(t, Path{"a", "b"}.IsParent(Path{"a", "c"}))
	assert.False(t, Path{"a"}.IsParent(Path{"a", "c"}))
}
