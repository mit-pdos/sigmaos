package fsux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
)

const (
	fn = "name/ux/~ip/"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	var err error
	ts := &Tstate{}
	ts.t = t
	ts.s, ts.FsLib, err = kernel.MakeSystemAll("fsux_test", "..")
	assert.Nil(t, err, "Start")
	return ts
}

func (ts *Tstate) Shutdown() {
	ts.s.Shutdown(ts.FsLib)
}

func TestRoot(t *testing.T) {
	ts := makeTstate(t)

	dirents, err := ts.ReadDir("name/ux/~ip/")
	assert.Nil(t, err, "ReadDir")

	assert.NotEqual(t, 0, len(dirents))

	// log.Printf("dirents %v\n", dirents)

	ts.Shutdown()
}

func TestFile(t *testing.T) {
	ts := makeTstate(t)

	d := []byte("hello")
	err := ts.MakeFile(fn+"f", 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.ReadFile(fn + "f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "f")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestDir(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(fn+"d1", 0777)
	assert.Equal(t, nil, err)
	d := []byte("hello")

	dirents, err := ts.ReadDir(fn + "d1")
	assert.Nil(t, err, "ReadDir")

	assert.Equal(t, 0, len(dirents))

	err = ts.MakeFile(fn+"d1/f", 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.ReadFile(fn + "d1/f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "d1/f")
	assert.Equal(t, nil, err)

	err = ts.Remove(fn + "d1")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}
