package sigmap

import (
	"errors"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEOF(t *testing.T) {
	err := MkErrError(io.EOF)
	assert.True(t, errors.Is(err, io.EOF))
}

func TestError(t *testing.T) {
	for c := TErrBadattach; c <= TErrError; c++ {
		log.Printf("%d %v\n", c, c)
		assert.True(t, c.String() != "unknown error", c)
	}
}

func TestString(t *testing.T) {
	qt := Qtype(QTSYMLINK | QTTMP)
	assert.Equal(t, qt.String(), "ts")

	p := Tperm(0x60001ff)
	assert.Equal(t, "qt ts qp ff", p.String())
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
	assert.True(t, Path{"a"}.IsParent(Path{}))
	assert.False(t, Path{"b"}.IsParent(Path{"a"}))
	assert.True(t, Path{"a", "b"}.IsParent(Path{"a"}))
	assert.False(t, Path{"a", "b"}.IsParent(Path{"a", "c"}))
	assert.False(t, Path{"a"}.IsParent(Path{"a", "c"}))
}
