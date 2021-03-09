package mr

import (
	// "fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

func RmDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	err = os.RemoveAll(dir)
	if err != nil {
		return err
	}
	return nil
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
	for r := 0; r < NReduce; r++ {
		s := strconv.Itoa(r)
		err = ts.Mkdir("name/fs/"+s, 0777)
		if err != nil {
			t.Fatalf("Mkdir %v\n", err)
		}
	}
	return ts
}

func TestWc(t *testing.T) {
	ts := makeTstate(t)
	mappers := []string{}
	n := 0
	files, err := ioutil.ReadDir("../input")
	if err != nil {
		log.Fatalf("Readdir %v\n", err)
	}
	for _, f := range files {
		pid1 := fslib.GenPid()
		pid2 := fslib.GenPid()
		m := strconv.Itoa(n)
		a1 := &fslib.Attr{pid1, "../bin/fsreader", "",
			[]string{"name/s3/~ip/input/" + f.Name(), m}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil}
		a2 := &fslib.Attr{pid2, "../bin/mr-m-wc", "",
			[]string{"name/" + m + "/pipe", m}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil}
		ts.Spawn(a1)
		ts.Spawn(a2)
		n += 1
		mappers = append(mappers, pid2)
	}

	reducers := []string{}
	for i := 0; i < NReduce; i++ {
		pid := fslib.GenPid()
		r := strconv.Itoa(i)
		a := &fslib.Attr{pid, "../bin/mr-r-wc", "",
			[]string{"name/fs/" + r, "name/fs/mr-out-" + r}, nil,
			nil, mappers}
		reducers = append(reducers, pid)
		ts.Spawn(a)
	}

	// Spawn noop lambda that is dependent on reducers
	pid := fslib.GenPid()
	ts.SpawnNoOp(pid, reducers)
	ts.Wait(pid)

	var b []byte
	for i := 0; i < NReduce; i++ {
		// XXX run as a lambda?
		data, err := ts.ReadFile("name/fs/mr-out-" + strconv.Itoa(i))
		assert.Nil(t, err, "Readfile")
		b = append(b, data...)
	}

	b1, err := ioutil.ReadFile("seq-mr.out")
	assert.Nil(t, err, "Readfile seq")

	assert.Equal(t, len(b), len(b1), "Output len")

	//for i, v := range b {
	//	assert.Equal(t, v, b1[i], fmt.Sprintf("Buf %v diff %v %v\n", i, v, b1[i]))
	//}

	// Delete intermediate files
	for i := 0; i < n; i++ {
		err := RmDir("/tmp/m-" + strconv.Itoa(i))
		assert.Nil(t, err, "RmDir")
	}

	ts.s.Shutdown(ts.FsLib)
}
