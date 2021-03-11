package locald

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *fslib.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := "../bin"
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	ts.FsLib = fslib.MakeFsLib("locald-test")
	ts.t = t

	return ts
}

func TestWait(t *testing.T) {
	ts := makeTstate(t)

	debug.SetDebug(false)

	pid := fslib.GenPid()
	//	a := &fslib.Attr{pid, "../bin/schedl", "", []string{"name/out", ""}, nil, nil, nil}
	ip, err := fsclnt.LocalIP()
	err = ts.MakeFile(fslib.LOCALD_ROOT+"/"+ip+"/"+pid, []byte("aaaaaaa"))

	//	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	log.Printf("Spawn %v\n", pid)

	//	ts.Wait(pid)
	//
	//	b, err := ts.ReadFile("name/out")
	//	assert.Nil(t, err, "ReadFile")
	//	assert.Equal(t, string(b), "hello", "Output")

	ts.s.Shutdown(ts.FsLib)
}
