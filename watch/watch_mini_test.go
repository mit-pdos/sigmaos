package watch_test

import (
	"path/filepath"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSend(t *testing.T) {
	dir := sp.NAMED
	ts, err := test.NewTstatePath(t, dir)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	testdir := filepath.Join(dir, "test")
	err = ts.MkDir(testdir, 0777)
	assert.Nil(t, err)

	fd, err := ts.Open(testdir, sp.OREAD)
	assert.Nil(t, err)

	watchfd, err := ts.DirWatchV2(fd)
	assert.Nil(t, err)

	go func() {
		_, err := ts.Create(filepath.Join(testdir, "testfile" + strconv.Itoa(0)), 0777, sp.OWRITE)
		assert.Nil(t, err)
	}()

	b := make([]byte, 1000)
	db.DPrintf(db.WATCH_NEW, "Reading watchfd %v", watchfd)
	sz, err := ts.Read(watchfd, b)
	assert.Nil(t, err)
	db.DPrintf(db.WATCH_NEW, "Read %v bytes: %s", sz, b[:sz])
	db.DPrintf(db.WATCH_NEW, "%v", b)

	ts.RmDirEntries(testdir)
	assert.Nil(t, err)

	err = ts.RmDir(testdir)
	assert.Nil(t, err)

	err = ts.CloseFd(watchfd)
	assert.Nil(t, err)

	err = ts.CloseFd(fd)
	assert.Nil(t, err)

	err = ts.Close()
	assert.Nil(t, err)

	ts.Shutdown()
}