package nps3

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/debug"
	"ulambda/fslib"
)

type Tstate struct {
	*fslib.FsLib
	t    *testing.T
	s    *fslib.System
	nps3 *Nps3
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	debug.SetDebug(true)

	bin := "../bin"
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	ts.nps3 = MakeNps3()

	ts.FsLib = fslib.MakeFsLib("nps3c")

	ts.t = t

	return ts
}

func TestRoot(t *testing.T) {
	ts := makeTstate(t)

	dirents, err := ts.ReadDir("name/s3")
	assert.Nil(t, err, "ReadDir")
	log.Printf("dirents: %v\n", dirents)

	ts.s.Shutdown(ts.FsLib)
}
