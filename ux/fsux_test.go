package fsux

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	np "sigmaos/ninep"
	"sigmaos/test"
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

func mkfile(t *testing.T, name string) {
	CNT := 500
	buf := test.MkBuf(test.BUFSZ)
	start := time.Now()
	fd, err := syscall.Open(name, syscall.O_CREAT|syscall.O_EXCL|syscall.O_WRONLY, 0666)
	assert.Nil(t, err)
	for i := 0; i < CNT; i++ {
		n, err := syscall.Pwrite(fd, buf, int64(i*test.BUFSZ))
		assert.Nil(t, err)
		assert.Equal(t, test.BUFSZ, n)
	}
	syscall.Fsync(fd)
	syscall.Close(fd)
	ms := time.Since(start).Milliseconds()
	sz := uint64(CNT * len(buf))
	fmt.Printf("%s took %vms (%s)\n", humanize.Bytes(sz), ms, test.TputStr(np.Tlength(sz), ms))
	os.Remove(name)
}

func TestFsPerfSingle(t *testing.T) {
	mkfile(t, "xxx")
}

func TestFsPerfMulti(t *testing.T) {

	var done sync.WaitGroup
	done.Add(2)
	go func() {
		mkfile(t, "xxx")
		done.Done()
	}()
	go func() {
		mkfile(t, "yyy")
		done.Done()
	}()
	done.Wait()
}
