package sigmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	qt := Qtype(QTSYMLINK | QTTMP)
	assert.Equal(t, qt.String(), "ts")

	p := Tperm(0x60001ff)
	assert.Equal(t, "qt ts qp ff", p.String())
}
