package ninep

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	qt := Qtype(QTSYMLINK | QTTMP)
	assert.Equal(t, qt.String(), "ts")

	p := Tperm(0x60001ff)
	assert.Equal(t, "qt ts p ff", p.String())
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
	assert.True(t, IsParent([]string{}, []string{"a"}))
	assert.False(t, IsParent([]string{"a"}, []string{"b"}))
	assert.True(t, IsParent([]string{"a"}, []string{"a", "b"}))
	assert.False(t, IsParent([]string{"a", "c"}, []string{"a", "b"}))
	assert.False(t, IsParent([]string{"a", "c"}, []string{"a"}))
}
