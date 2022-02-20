package reader_test

import (
	"io"
	"testing"
	"ulambda/reader"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
)

type Tstate struct {
	t *testing.T
	*kernel.System
	replicas []*kernel.System
}

func (ts *Tstate) Shutdown() {
	ts.System.Shutdown()
	for _, r := range ts.replicas {
		r.Shutdown()
	}
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemNamed("fslibtest", "..", 0)
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		ts.replicas = append(ts.replicas, kernel.MakeSystemNamed("fslibtest", "..", i+1))
	}
	return ts
}

func TestReader1(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/f"
	d := []byte("abcdefg")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := reader.MakeReader(ts.FsLib, fn)

	v := make([]byte, 1)
	for _, b := range d {
		n, err := rdr.Read(v)
		assert.Equal(ts.t, nil, err)
		assert.Equal(ts.t, 1, n)
		assert.Equal(ts.t, b, v[0])
	}
	n, err := rdr.Read(v)
	assert.Equal(ts.t, io.EOF, err)
	assert.Equal(ts.t, 0, n)

	ts.Shutdown()

}

func TestReader2(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/f"
	d := []byte("a")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := reader.MakeReader(ts.FsLib, fn)

	v := make([]byte, 2)
	n, err := rdr.Read(v)
	assert.Equal(ts.t, nil, err)
	assert.Equal(ts.t, 1, n)
	assert.Equal(ts.t, d[0], v[0])
	n, err = rdr.Read(v)
	assert.Equal(ts.t, io.EOF, err)
	assert.Equal(ts.t, 0, n)

	ts.Shutdown()
}

func TestReaderLarge(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/f"
	ts.SetChunkSz(4096)
	sz := int(2*ts.GetChunkSz()) + 1
	d := make([]byte, sz)
	for i := 0; i < sz; i++ {
		d[i] = byte(i)
	}
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := reader.MakeReader(ts.FsLib, fn)

	n := 0
	for {
		v := make([]byte, 9)
		m, err := rdr.Read(v)
		if err != nil {
			assert.Equal(ts.t, io.EOF, err)
			break
		}
		for j := 0; j < m; j++ {
			assert.Equal(ts.t, d[n], v[j])
			n += 1
		}
	}
	assert.Equal(ts.t, sz, n)
	ts.Shutdown()
}
