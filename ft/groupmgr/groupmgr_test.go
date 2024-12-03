package groupmgr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestREPL(t *testing.T) {
	s := newREPL(1, 2)
	id, n, err := ParseREPL(s)
	assert.Nil(t, err)
	assert.Equal(t, 1, id)
	assert.Equal(t, 2, n)
}
