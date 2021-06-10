package mr

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
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
		err = os.RemoveAll(path.Join(dir, name))
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

	s, err := fslib.Boot("..")
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

func rmDir(ts *Tstate, dir string) {
	fs, err := ts.ReadDir(dir)
	if err != nil {
		log.Printf("Couldn't read dir during rmDir: %v, %v", dir, err)
	}
	for _, f := range fs {
		err = ts.Remove(path.Join(dir, f.Name))
		if err != nil {
			log.Printf("Couldn't remove: %v", err)
		}
	}
	err = ts.Remove(dir)
	if err != nil {
		log.Printf("Couldn't remove: %v", err)
	}
}

func TestWc(t *testing.T) {
	ts := makeTstate(t)
	mappers := map[string]bool{}
	n := 0
	files, err := ioutil.ReadDir("../input")
	if err != nil {
		log.Fatalf("Readdir %v\n", err)
	}
	for _, f := range files {
		pid1 := fslib.GenPid()
		pid2 := fslib.GenPid()
		m := strconv.Itoa(n)
		rmDir(ts, "name/ux/~ip/m-"+m)
		a1 := &fslib.Attr{pid1, "bin/fsreader", "",
			[]string{"name/s3/~ip/input/" + f.Name(), m}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil, 0, fslib.DEFP,
			fslib.DEFC}
		a2 := &fslib.Attr{pid2, "bin/mr-m-wc", "",
			[]string{"name/" + m + "/pipe", m}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil, 0, fslib.DEFP,
			fslib.DEFC}
		ts.Spawn(a1)
		ts.Spawn(a2)
		n += 1
		mappers[pid2] = false
	}

	reducers := []string{}
	for i := 0; i < NReduce; i++ {
		pid := fslib.GenPid()
		r := strconv.Itoa(i)
		a := &fslib.Attr{pid, "bin/mr-r-wc", "",
			[]string{"name/fs/" + r, "name/fs/mr-out-" + r}, nil,
			nil, mappers, 0, fslib.DEFP, fslib.DEFC}
		reducers = append(reducers, pid)
		ts.Spawn(a)
	}

	// Spawn noop lambda that is dependent on reducers
	pid := fslib.GenPid()
	ts.SpawnNoOp(pid, reducers)
	ts.Wait(pid)

	file, err := os.OpenFile("par-mr.out", os.O_WRONLY|os.O_CREATE, 0644)
	assert.Nil(t, err, "OpenFile")

	defer file.Close()
	for i := 0; i < NReduce; i++ {
		// XXX run as a lambda?
		r := strconv.Itoa(i)
		data, err := ts.ReadFile("name/fs/mr-out-" + r)
		assert.Nil(t, err, "Readfile")
		_, err = file.Write(data)
		assert.Nil(t, err, "Write")
	}

	Compare(t)

	// Delete intermediate files
	for i := 0; i < n; i++ {
		err := RmDir("/tmp/m-" + strconv.Itoa(i))
		assert.Nil(t, err, "RmDir")
	}

	ts.s.Shutdown(ts.FsLib)
}

func Compare(t *testing.T) {
	cmd := exec.Command("sort", "seq-mr.out")
	var out1 bytes.Buffer
	cmd.Stdout = &out1
	err := cmd.Run()
	if err != nil {
		log.Printf("err %v\n", err)
	}
	cmd = exec.Command("sort", "par-mr.out")
	var out2 bytes.Buffer
	cmd.Stdout = &out2
	err = cmd.Run()
	if err != nil {
		log.Printf("err %v\n", err)
	}
	b1 := out1.Bytes()
	b2 := out2.Bytes()
	assert.Equal(t, len(b1), len(b2), "Output len")
	for i, v := range b1 {
		assert.Equal(t, v, b2[i], fmt.Sprintf("Buf %v diff %v %v\n", i, v, b2[i]))
	}

}
