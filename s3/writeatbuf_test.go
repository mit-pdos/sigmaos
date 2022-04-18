package fss3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	// db "ulambda/debug"
	np "ulambda/ninep"
)

func cmp(b0 []byte, b1 []byte) bool {
	if len(b0) != len(b1) {
		return false
	}
	for i := 0; i < len(b0); i++ {
		if b0[i] != b1[i] {
			return false
		}
	}
	return true
}

func TestBuf(t *testing.T) {
	const (
		N  = 1000
		WS = 100
	)
	wb := mkWriteAtBuffer(N)
	buf := make([]byte, WS)
	for i := 0; i < WS; i++ {
		wb.tb.b[i] = byte(i & 0xFF)
	}
	wb.WriteAt(buf, 0)
	d, err := wb.read(0, WS)
	assert.Nil(t, err)
	assert.True(t, cmp(buf, d))

	off := np.Toffset(len(buf))
	wb.WriteAt(buf, int64(len(buf)))
	d, err = wb.read(off, WS)
	assert.Nil(t, err)
	assert.True(t, cmp(buf, d))
}
