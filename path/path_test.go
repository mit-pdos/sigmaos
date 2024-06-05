package path

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompile(t *testing.T) {
}

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
	assert.True(t, Tpathname{"a"}.IsParent(Tpathname{}))
	assert.False(t, Tpathname{"b"}.IsParent(Tpathname{"a"}))
	assert.True(t, Tpathname{"a", "b"}.IsParent(Tpathname{"a"}))
	assert.False(t, Tpathname{"a", "b"}.IsParent(Tpathname{"a", "c"}))
	assert.False(t, Tpathname{"a"}.IsParent(Tpathname{"a", "c"}))
}
