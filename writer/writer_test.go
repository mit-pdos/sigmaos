package writer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	np "sigmaos/sigmap"
	"sigmaos/test"
)

func TestWriter1(t *testing.T) {
	ts := test.MakeTstate(t)

	fn := "name/f"
	d := []byte("abcdefg")
	wrt, err := ts.CreateWriter(fn, 0777, np.OWRITE)
	assert.Nil(ts.T, err)

	for _, b := range d {
		v := make([]byte, 1)
		v[0] = b
		n, err := wrt.Write(v)
		assert.Equal(ts.T, nil, err)
		assert.Equal(ts.T, 1, n)
	}
	wrt.Close()

	d1, err := ts.GetFile(fn)
	assert.Equal(t, d, d1)

	ts.Shutdown()
}
