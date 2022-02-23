package fss3

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fsclnt"
	np "ulambda/ninep"
	"ulambda/test"
)

func TestOne(t *testing.T) {
	ts := test.MakeTstateAll(t)

	dirents, err := ts.GetDir("name/s3/")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 1, len(dirents))

	ts.Shutdown()
}

func TestTwo(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a second one
	ts.BootFss3d()

	time.Sleep(100 * time.Millisecond)

	dirents, err := ts.GetDir("name/s3")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 2, len(dirents))

	ts.Shutdown()
}

func TestUnionSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a second one
	ts.BootFss3d()

	dirents, err := ts.GetDir("name/s3/~ip/")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 5, len(dirents))

	ts.Shutdown()
}

func TestUnionDir(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a second one
	ts.BootFss3d()

	dirents, err := ts.GetDir("name/s3/~ip/input")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 8, len(dirents))

	ts.Shutdown()
}

func TestUnionFile(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a second one
	ts.BootFss3d()

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
	assert.Equal(ts.T, int(st.Length), n)

	ts.Shutdown()
}

func TestStat(t *testing.T) {
	ts := test.MakeTstateAll(t)

	name := "name/s3/~ip/input/pg-being_ernest.txt"
	st, err := ts.Stat(name)
	assert.Nil(t, err, "Stat")

	addr, err := fsclnt.LocalIP()
	assert.Nil(t, err, "LocalIP")
	st, err = ts.Stat("name/s3/~ip")
	assert.Nil(t, err, "Stat~")
	a := strings.Split(st.Name, ":")[0]
	assert.Equal(t, addr, a)

	ts.Shutdown()
}

func s3Name(ts *test.Tstate) string {
	sts, err := ts.GetDir("name/s3/")
	assert.Nil(ts.T, err, "name/s3")
	assert.Equal(ts.T, 1, len(sts))
	name := "name/s3" + "/" + sts[0].Name
	return name
}

func TestSymlinkFile(t *testing.T) {
	ts := test.MakeTstateAll(t)

	dn := s3Name(ts)
	fn := dn + "/b.txt"

	_, err := ts.GetFile(fn)
	assert.Nil(t, err, "GetFile")

	fn = dn + "//b.txt"
	_, err = ts.GetFile(fn)
	assert.Nil(t, err, "GetFile")

	ts.Shutdown()
}

func TestSymlinkDir(t *testing.T) {
	ts := test.MakeTstateAll(t)

	dn := s3Name(ts)

	b, err := ts.GetFile(dn)
	assert.Nil(t, err, "GetFile")
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))

	dirents, err := ts.GetDir(dn + "/")
	assert.Nil(t, err, "GetDir")
	assert.Equal(t, 5, len(dirents))

	ts.Shutdown()
}
