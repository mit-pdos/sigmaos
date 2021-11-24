package fsux

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
	"ulambda/realm"
)

const (
	bin = ".."
	fn  = "name/ux/~ip/"
)

type Tstate struct {
	*fslib.FsLib
	t   *testing.T
	s   *kernel.System
	e   *realm.TestEnv
	cfg *realm.RealmConfig
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	ts.t = t

	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg
	ts.s = kernel.MakeSystem(bin, cfg.NamedAddr)

	db.Name("fsux_test")
	ts.FsLib = fslib.MakeFsLibAddr("fsux_test", cfg.NamedAddr)

	return ts
}

func TestRoot(t *testing.T) {
	ts := makeTstate(t)

	dirents, err := ts.ReadDir("name/ux/~ip/")
	assert.Nil(t, err, "ReadDir")

	assert.NotEqual(t, 0, len(dirents))

	// log.Printf("dirents %v\n", dirents)

	ts.s.Shutdown()
	ts.e.Shutdown()
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

	ts.s.Shutdown()
	ts.e.Shutdown()
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

	ts.s.Shutdown()
	ts.e.Shutdown()
}
