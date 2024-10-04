package reader_test

import (
	"flag"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "namedv1/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestCompile(t *testing.T) {
}

func TestReader1(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
	d := []byte("abcdefg")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)
	v := make([]byte, 1)
	for _, b := range d {
		n, err := rdr.Reader.Read(v)
		assert.Equal(ts.T, nil, err)
		assert.Equal(ts.T, 1, n)
		assert.Equal(ts.T, b, v[0])
	}
	n, err := rdr.Reader.Read(v)
	assert.Equal(ts.T, io.EOF, err)
	assert.Equal(ts.T, 0, n)
	rdr.Close()

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestReader2(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
	d := []byte("a")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)
	v := make([]byte, 2)
	n, err := rdr.Reader.Read(v)
	assert.Equal(ts.T, nil, err)
	assert.Equal(ts.T, 1, n)
	assert.Equal(ts.T, d[0], v[0])
	n, err = rdr.Reader.Read(v)
	assert.Equal(ts.T, io.EOF, err)
	assert.Equal(ts.T, 0, n)
	rdr.Close()

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestReaderLarge(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
	// ts.SetChunkSz(4096)
	sz := 4096 // int(2*ts.GetChunkSz()) + 1
	d := make([]byte, sz)
	for i := 0; i < sz; i++ {
		d[i] = byte(i)
	}
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)
	n := 0
	for {
		v := make([]byte, 9)
		m, err := rdr.Reader.Read(v)
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

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}
