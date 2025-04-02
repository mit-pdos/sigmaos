package path_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/path"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
	assert.NotNil(t, test.User)
}

func TestSplit(t *testing.T) {
	s := path.Split("name/s3/192.168.2.114:43471//b.txt")
	assert.Equal(t, 4, len(s))
	assert.Equal(t, s[3], "b.txt")

	s = path.Split("name/s3/192.168.2.114:43471//b.txt/")
	assert.Equal(t, 4, len(s))
	assert.Equal(t, s[3], "b.txt")

	s = path.Split("name/s3////192.168.2.114:43471//b.txt/")
	assert.Equal(t, 4, len(s))
	assert.Equal(t, s[3], "b.txt")
}

func TestIsParent(t *testing.T) {
	assert.True(t, path.Tpathname{"a"}.IsParent(path.Tpathname{}))
	assert.False(t, path.Tpathname{"b"}.IsParent(path.Tpathname{"a"}))
	assert.True(t, path.Tpathname{"a", "b"}.IsParent(path.Tpathname{"a"}))
	assert.False(t, path.Tpathname{"a", "b"}.IsParent(path.Tpathname{"a", "c"}))
	assert.False(t, path.Tpathname{"a"}.IsParent(path.Tpathname{"a", "c"}))
}
