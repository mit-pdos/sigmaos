package reader_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	np "sigmaos/sigmap"
	"sigmaos/test"
)

func TestReader1(t *testing.T) {
	ts := test.MakeTstate(t)

	fn := "name/f"
	d := []byte("abcdefg")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)
	v := make([]byte, 1)
	for _, b := range d {
		n, err := rdr.Read(v)
		assert.Equal(ts.T, nil, err)
		assert.Equal(ts.T, 1, n)
		assert.Equal(ts.T, b, v[0])
	}
	n, err := rdr.Read(v)
	assert.Equal(ts.T, io.EOF, err)
	assert.Equal(ts.T, 0, n)
	rdr.Close()
	ts.Shutdown()
}

func TestReader2(t *testing.T) {
	ts := test.MakeTstate(t)

	fn := "name/f"
	d := []byte("a")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)
	v := make([]byte, 2)
	n, err := rdr.Read(v)
	assert.Equal(ts.T, nil, err)
	assert.Equal(ts.T, 1, n)
	assert.Equal(ts.T, d[0], v[0])
	n, err = rdr.Read(v)
	assert.Equal(ts.T, io.EOF, err)
	assert.Equal(ts.T, 0, n)
	rdr.Close()
	ts.Shutdown()
}

func TestReaderLarge(t *testing.T) {
	ts := test.MakeTstate(t)

	fn := "name/f"
	ts.SetChunkSz(4096)
	sz := int(2*ts.GetChunkSz()) + 1
	d := make([]byte, sz)
	for i := 0; i < sz; i++ {
		d[i] = byte(i)
	}
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)
	n := 0
	for {
		v := make([]byte, 9)
		m, err := rdr.Read(v)
		if err != nil {
			assert.Equal(ts.T, io.EOF, err)
			break
		}
		for j := 0; j < m; j++ {
			assert.Equal(ts.T, d[n], v[j])
			n += 1
		}
	}
	assert.Equal(ts.T, sz, n)
	rdr.Close()
	ts.Shutdown()
}
