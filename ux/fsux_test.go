package fsux

import (
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
	"ulambda/test"
)

const (
	fn = np.UX + "/~ip/"
)

func TestRoot(t *testing.T) {
	ts := test.MakeTstateAll(t)

	dirents, err := ts.GetDir(fn)
	assert.Nil(t, err, "GetDir")

	assert.NotEqual(t, 0, len(dirents))

	ts.Shutdown()
}

func TestFile(t *testing.T) {
	ts := test.MakeTstateAll(t)

	d := []byte("hello")
	_, err := ts.PutFile(fn+"f", 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(fn + "f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "f")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestDir(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(fn+"d1", 0777)
	assert.Equal(t, nil, err)
	d := []byte("hello")

	dirents, err := ts.GetDir(fn + "d1")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 0, len(dirents))

	_, err = ts.PutFile(fn+"d1/f", 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(fn + "d1/f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "d1/f")
	assert.Equal(t, nil, err)

	err = ts.Remove(fn + "d1")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestFsPerf(t *testing.T) {
	CNT := 500
	N := 1 * test.MBYTE
	buf := test.MkBuf(N)
	start := time.Now()
	fd, err := syscall.Open("xxx", syscall.O_CREAT|syscall.O_EXCL|syscall.O_WRONLY, 0)
	assert.Nil(t, err)
	for i := 0; i < CNT; i++ {
		n, err := syscall.Pwrite(fd, buf, int64(i*N))
		assert.Nil(t, err)
		assert.Equal(t, N, n)
	}
	syscall.Close(fd)
	ms := time.Since(start).Milliseconds()
	sz := uint64(CNT * len(buf))
	fmt.Printf("%s took %vms (%s)", humanize.Bytes(sz), ms, test.TputStr(np.Tlength(sz), ms))
}
