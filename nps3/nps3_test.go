package nps3

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
)

type Tstate struct {
	*fslib.FsLib
	t    *testing.T
	s    *kernel.System
	nps3 *Nps3
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	ts.t = t

	bin := ".."
	s, err := kernel.BootMin(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	s.BootNps3d(bin)
	ts.s = s
	ts.FsLib = fslib.MakeFsLib("nps3_test")
	db.Name("nps3_test")
	return ts
}

func TestOne(t *testing.T) {
	ts := makeTstate(t)

	dirents, err := ts.ReadDir("name/s3/")
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

func TestUnionSimple(t *testing.T) {
	ts := makeTstate(t)

	// Make a second one
	ts.nps3 = MakeNps3()
	go ts.nps3.Serve()

	dirents, err := ts.ReadDir("name/s3/~ip/")
	assert.Nil(t, err, "ReadDir")

	assert.Equal(t, 5, len(dirents))

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

func (ts *Tstate) s3Name(t *testing.T) string {
	sts, err := ts.ReadDir("name/s3/")
	assert.Nil(t, err, "name/s3")
	assert.Equal(t, 1, len(sts))
	name := "name/s3" + "/" + sts[0].Name
	return name
}

func TestSymlinkFile(t *testing.T) {
	ts := makeTstate(t)

	dn := ts.s3Name(t)
	fn := dn + "/b.txt"

	_, err := ts.ReadFile(fn)
	assert.Nil(t, err, "ReadFile")

	fn = dn + "//b.txt"
	_, err = ts.ReadFile(fn)
	assert.Nil(t, err, "ReadFile")

	ts.s.Shutdown(ts.FsLib)
}

func TestSymlinkDir(t *testing.T) {
	ts := makeTstate(t)

	dn := ts.s3Name(t)

	b, err := ts.ReadFile(dn)
	assert.Nil(t, err, "ReadFile")
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))

	dirents, err := ts.ReadDir(dn + "/")
	assert.Nil(t, err, "ReadDir")
	assert.Equal(t, 5, len(dirents))

	ts.s.Shutdown(ts.FsLib)
}
