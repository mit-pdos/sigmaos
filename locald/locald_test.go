package locald

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	// "ulambda/debug"
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

	ts.FsLib = fslib.MakeFsLib("schedl")
	ts.t = t

	return ts
}

func TestSpawn(t *testing.T) {
	ts := makeTstate(t)

	pid := fslib.GenPid()
	a := &fslib.Attr{pid, "bin/schedl", "", []string{"name/out", ""}, nil, nil, nil}

	b, err := json.Marshal(a)

	assert.Nil(t, err, "Marshal")

	err = ts.MakeFile(fslib.LOCALD_ROOT+"/~ip/"+pid, b)

	assert.Nil(t, err, "Spawn")

	//	ts.Wait(pid)
	//
	//	b, err := ts.ReadFile("name/out")
	//	assert.Nil(t, err, "ReadFile")
	//	assert.Equal(t, string(b), "hello", "Output")

	ts.s.Shutdown(ts.FsLib)

	//	a := &fslib.Attr{pid, "echo", "", []string{"name/out", ""}, nil, nil, nil}
	//
	//	err := ts.Spawn(a)
	//	assert.Nil(t, err, "Spawn")
	//
	//	ts.Wait(pid)
	//
	//	b, err := ts.ReadFile("name/out")
	//	assert.Nil(t, err, "ReadFile")
	//	assert.Equal(t, string(b), "hello", "Output")
	//
	//	ts.s.Shutdown(ts.FsLib)
}
