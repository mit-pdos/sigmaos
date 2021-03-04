package mr

import (
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
)

type Tstate struct {
	*fslib.FsLib
	t    *testing.T
	s    *fslib.System
	done chan bool
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.done = make(chan bool)

	bin := "../bin"
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	ts.FsLib = fslib.MakeFsLib("boot")
	err = ts.Mkdir("name/fs", 0777)
	if err != nil {
		t.Fatalf("Mkdir %v\n", err)
	}
	return ts
}

func TestWc(t *testing.T) {
	ts := makeTstate(t)
	pids := []string{}
	n := 0
	files, err := ioutil.ReadDir("../input")
	if err != nil {
		log.Fatalf("Readdir %v\n", err)
	}
	for _, f := range files {
		pid1 := fslib.GenPid()
		pid2 := fslib.GenPid()
		s := strconv.Itoa(n)
		err = ts.Mkdir("name/fs/"+s, 0777)
		if err != nil {
			t.Fatalf("Mkdir %v\n", err)
		}
		a1 := &fslib.Attr{pid1, "../bin/fsreader", "",
			[]string{"name/s3/~ip/input/" + f.Name(), s}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil}
		a2 := &fslib.Attr{pid2, "../bin/mr-m-wc", "",
			[]string{"name/" + s + "/pipe", s}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil}
		ts.Spawn(a1)
		ts.Spawn(a2)
		n += 1
		pids = append(pids, pid2)
	}
	// only one reducer
	pid := fslib.GenPid()
	a := &fslib.Attr{pid, "../bin/mr-r-wc", "",
		[]string{"name/fs/0", "name/fs/mr-out"}, nil,
		nil, pids}
	ts.Spawn(a)

	ts.Wait(pid)

	// XXX run as a lambda?
	b, err := ts.ReadFile("name/fs/mr-out")
	assert.Nil(t, err, "Readfile")

	b1, err := ioutil.ReadFile("seq-mr.out")
	assert.Nil(t, err, "Readfile seq")

	assert.Equal(t, len(b), len(b1), "Output len")

	for i, v := range b {
		assert.Equal(t, v, b1[i], fmt.Sprintf("Buf %v diff %v %v\n", i, v, b1[i]))
	}

	ts.s.Shutdown(ts.FsLib)
}
