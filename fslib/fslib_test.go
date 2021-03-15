package fslib

import (
	// "log"
	"testing"

	"github.com/stretchr/testify/assert"
	db "ulambda/debug"
	"ulambda/fsclnt"
)

type Tstate struct {
	*FsLib
	t *testing.T
	s *System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	s, err := Boot("..")
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.FsLib = MakeFsLib("fslib")
	ts.s = s
	ts.t = t
	return ts
}

func TestSymlink(t *testing.T) {
	ts := makeTstate(t)

	db.SetDebug(false)
	b, err := ts.ReadFile("name/schedd")
	assert.Nil(t, err, "named/schedd")
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))

	sts, err := ts.ReadDir("name/schedd/")
	assert.Nil(t, err, "named/schedd/")
	// log.Printf("stats: %v\n", sts)
	assert.Equal(t, 0, len(sts))

	ts.s.Shutdown(ts.FsLib)
}
