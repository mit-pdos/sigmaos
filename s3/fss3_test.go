package fss3

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fidclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/pathclnt"
	"ulambda/test"
)

var ROOT = []string{np.STATSD, "a", "b.txt", "gutenberg", "wiki", "ls.PDF"}

func TestOne(t *testing.T) {
	ts := test.MakeTstateAll(t)

	dirents, err := ts.GetDir("name/s3/")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 1, len(dirents))

	ts.Shutdown()
}

func TestReadOff(t *testing.T) {
	ts := test.MakeTstateAll(t)

	rdr, err := ts.OpenReader("name/s3/~ip/gutenberg/pg-being_ernest.txt")
	assert.Equal(t, nil, err)
	rdr.Lseek(1 << 10)
	brdr := bufio.NewReaderSize(rdr, 1<<16)
	scanner := bufio.NewScanner(brdr)
	l := np.Tlength(1 << 10)
	n := 0
	for scanner.Scan() {
		line := scanner.Text()
		n += len(line) + 1 // 1 for newline
		if np.Tlength(n) > l {
			break
		}
	}
	assert.Equal(t, 1072, n)

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

	assert.True(t, fslib.Present(dirents, ROOT))

	ts.Shutdown()
}

func TestUnionDir(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a second one
	ts.BootFss3d()

	dirents, err := ts.GetDir("name/s3/~ip/gutenberg")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 8, len(dirents))

	ts.Shutdown()
}

func TestUnionFile(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a second one
	ts.BootFss3d()

	file, err := os.ReadFile("../input/pg-being_ernest.txt")
	assert.Nil(t, err, "ReadFile")

	name := "name/s3/~ip/gutenberg/pg-being_ernest.txt"
	st, err := ts.Stat(name)
	assert.Nil(t, err, "Stat")

	fd, err := ts.Open(name, np.OREAD)
	if err != nil {
		db.DFatalf("%v", err)
	}
	n := len(file)
	for {
		data, err := ts.Read(fd, 8192)
		if len(data) == 0 {
			break
		}
		if err != nil {
			db.DFatalf("%v", err)
		}
		for i := 0; i < len(data); i++ {
			assert.Equal(t, file[i], data[i])
		}
		file = file[len(data):]
	}
	assert.Equal(ts.T, int(st.Length), n)

	ts.Shutdown()
}

func TestStat(t *testing.T) {
	ts := test.MakeTstateAll(t)

	name := "name/s3/~ip/gutenberg/pg-being_ernest.txt"
	st, err := ts.Stat(name)
	assert.Nil(t, err, "Stat")

	addr, err := fidclnt.LocalIP()
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
	assert.Equal(t, true, pathclnt.IsRemoteTarget(string(b)))

	dirents, err := ts.GetDir(dn + "/")
	assert.Nil(t, err, "GetDir")

	assert.True(t, fslib.Present(dirents, ROOT))

	ts.Shutdown()
}
