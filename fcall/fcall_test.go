package fcall

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
