package serr_test

import (
	"errors"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/serr"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
	assert.NotNil(t, test.User)
}

func TestEOF(t *testing.T) {
	err := serr.NewErrError(io.EOF)
	assert.True(t, errors.Is(err, io.EOF))
}

func f() error {
	return serr.NewErr(serr.TErrNotfound, "f")
}

func TestErr(t *testing.T) {
	var err error
	assert.False(t, serr.IsErrCode(err, serr.TErrNotfound))
	err = f()
	assert.NotNil(t, err)
	assert.True(t, serr.IsErrCode(err, serr.TErrNotfound))
}

func TestError(t *testing.T) {
	for c := serr.Terror(serr.TErrBadattach); c <= serr.TErrError; c = serr.Terror(c + 1) {
		log.Printf("%d %v\n", c, c)
		assert.True(t, c.String() != "unknown error", c)
	}
}
