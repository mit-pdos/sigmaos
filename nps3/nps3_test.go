package nps3

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/debug"
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

	debug.SetDebug(false)

	bin := "../bin"
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	ts.FsLib = fslib.MakeFsLib("nps3c")

	return ts
}

func TestOne(t *testing.T) {
	ts := makeTstate(t)

	// Boot makes one nps3d

	time.Sleep(100 * time.Millisecond)

	dirents, err := ts.ReadDir("name/s3")
	assert.Nil(t, err, "ReadDir")

	assert.Equal(t, 1, len(dirents))

	log.Printf("shutdown\n")
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
	log.Printf("dirents: %v\n", dirents)

	assert.Equal(t, 2, len(dirents))

	log.Printf("shutdown\n")
	ts.s.Shutdown(ts.FsLib)
}

func TestUnion(t *testing.T) {
	ts := makeTstate(t)

	// Make a second one
	ts.nps3 = MakeNps3()
	go ts.nps3.Serve()

	dirents, err := ts.ReadDir("name/s3/~ip")
	assert.Nil(t, err, "ReadDir")

	log.Printf("dirents: %v\n", dirents)
	assert.Equal(t, 4, len(dirents))

	log.Printf("shutdown\n")
	ts.s.Shutdown(ts.FsLib)
}

func TestUnionDir(t *testing.T) {
	ts := makeTstate(t)

	// Make a second one
	ts.nps3 = MakeNps3()
	go ts.nps3.Serve()

	dirents, err := ts.ReadDir("name/s3/~ip/input")
	assert.Nil(t, err, "ReadDir")

	log.Printf("dirents: %v\n", dirents)
	assert.Equal(t, 8, len(dirents))

	log.Printf("shutdown\n")
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

	log.Printf("st %v\n", st)

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

	log.Printf("shutdown\n")
	ts.s.Shutdown(ts.FsLib)
}
