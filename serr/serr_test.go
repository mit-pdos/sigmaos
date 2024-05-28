package serr

import (
	"errors"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompile(t *testing.T) {
}

func TestEOF(t *testing.T) {
	err := NewErrError(io.EOF)
	assert.True(t, errors.Is(err, io.EOF))
}

func f() error {
	return NewErr(TErrNotfound, "f")
}

func TestErr(t *testing.T) {
	var err error
	assert.False(t, IsErrCode(err, TErrNotfound))
	err = f()
	assert.NotNil(t, err)
	assert.True(t, IsErrCode(err, TErrNotfound))
}

func TestErrString(t *testing.T) {
	err := NewErrString(`"{Err: "Non-sigma error" Obj: "" (exit status 2)}`)
	assert.True(t, err.ErrCode == TErrError)
}

func TestError(t *testing.T) {
	for c := TErrBadattach; c <= TErrError; c++ {
		log.Printf("%d %v\n", c, c)
		assert.True(t, c.String() != "unknown error", c)
	}
}
