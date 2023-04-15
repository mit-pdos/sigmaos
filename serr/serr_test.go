package serr

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

func f() error {
	return MkErr(TErrNotfound, "f")
}

func TestErr(t *testing.T) {
	var serr *Err
	var err error
	assert.False(t, errors.As(err, &serr) && serr.IsErrNotfound())
	err = f()
	assert.NotNil(t, err)
	assert.True(t, errors.As(err, &serr) && serr.IsErrNotfound())

}

func TestError(t *testing.T) {
	for c := TErrBadattach; c <= TErrError; c++ {
		log.Printf("%d %v\n", c, c)
		assert.True(t, c.String() != "unknown error", c)
	}
}
