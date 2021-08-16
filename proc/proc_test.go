package proc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *fslib.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("sync_test")

	ts.FsLib = fslib.MakeFsLib("sync_test")
	ts.t = t
	return ts
}

func TestHelloWorld(t *testing.T) {
	ts := makeTstate(t)

	assert.True(ts.t, true, "test")
	MakeProcCtl(ts.FsLib, "test-pid")

	ts.s.Shutdown(ts.FsLib)
}
