package nps3

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type Tstate struct {
	*fslib.FsLib
	t    *testing.T
	s    *fslib.System
	nps3 *Nps3
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	ts.t = t

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	ts.FsLib = fslib.MakeFsLib("nps3c_test")

	return ts
}

func TestOne(t *testing.T) {
	ts := makeTstate(t)

	// Boot makes one nps3d

	time.Sleep(100 * time.Millisecond)

	dirents, err := ts.ReadDir("name/s3")
	assert.Nil(t, err, "ReadDir")

	assert.Equal(t, 1, len(dirents))

	ts.s.Shutdown(ts.FsLib)
}

func TestTwo(t *testing.T) {
	ts := makeTstate(t)

	// Make a second one
	ts.nps3 = MakeNps3()
	go ts.nps3.Serve()

	time.Sleep(100 * time.Millisecond)

	dirents, err := ts.ReadDir("name/s3")
	assert.Nil(t, err, "ReadDir")

	assert.Equal(t, 2, len(dirents))

	ts.s.Shutdown(ts.FsLib)
}

func TestUnion(t *testing.T) {
	ts := makeTstate(t)

	// Make a second one
	ts.nps3 = MakeNps3()
	go ts.nps3.Serve()

	dirents, err := ts.ReadDir("name/s3/~ip/")
	assert.Nil(t, err, "ReadDir")

	assert.Equal(t, 4, len(dirents))

	ts.s.Shutdown(ts.FsLib)
}

func TestUnionDir(t *testing.T) {
	ts := makeTstate(t)

	// Make a second one
	ts.nps3 = MakeNps3()
	go ts.nps3.Serve()

	dirents, err := ts.ReadDir("name/s3/~ip/input")
	assert.Nil(t, err, "ReadDir")

	assert.Equal(t, 8, len(dirents))

	ts.s.Shutdown(ts.FsLib)
}

func TestUnionFile(t *testing.T) {
	ts := makeTstate(t)

	// Make a second one
	ts.nps3 = MakeNps3()
	go ts.nps3.Serve()

	name := "name/s3/~ip/input/pg-being_ernest.txt"
	st, err := ts.Stat(name)
	assert.Nil(t, err, "Stat")

	fd, err := ts.Open(name, np.OREAD)
	if err != nil {
		log.Fatal(err)
	}
	n := 0
	for {
		data, err := ts.Read(fd, 8192)
		if len(data) == 0 {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		n += len(data)
	}
	assert.Equal(ts.t, int(st.Length), n)

	ts.s.Shutdown(ts.FsLib)
}

func TestStat(t *testing.T) {
	ts := makeTstate(t)

	name := "name/s3/~ip/input/pg-being_ernest.txt"
	st, err := ts.Stat(name)
	assert.Nil(t, err, "Stat")

	addr, err := fsclnt.LocalIP()
	assert.Nil(t, err, "LocalIP")
	st, err = ts.Stat("name/s3/~ip")
	assert.Nil(t, err, "Stat~")
	a := strings.Split(st.Name, ":")[0]
	assert.Equal(t, addr, a)

	ts.s.Shutdown(ts.FsLib)
}
