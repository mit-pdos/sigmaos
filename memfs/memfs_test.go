package memfs_test

import (
	"flag"
	gopath "path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/fslibtest"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestPipeBasic(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	pipe := gopath.Join(pathname, "pipe")
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func() {
		fsl, err := fslib.MakeFsLibAddr("reader", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
		fd, err := fsl.Open(pipe, sp.OREAD)
		assert.Nil(ts.T, err, "Open")
		b, err := fsl.Read(fd, 100)
		assert.Nil(ts.T, err, "Read")
		assert.Equal(ts.T, "hello", string(b))
		err = fsl.Close(fd)
		assert.Nil(ts.T, err, "Close")
		ch <- true
	}()
	fd, err := ts.Open(pipe, sp.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.Close(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Remove(pipe)

	ts.Shutdown()
}

func TestPipeClose(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	pipe := gopath.Join(pathname, "pipe")
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		fsl, err := fslib.MakeFsLibAddr("reader", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
		fd, err := fsl.Open(pipe, sp.OREAD)
		assert.Nil(ts.T, err, "Open")
		for true {
			b, err := fsl.Read(fd, 100)
			if err != nil { // writer closed pipe
				break
			}
			assert.Nil(ts.T, err, "Read")
			assert.Equal(ts.T, "hello", string(b))
		}
		err = fsl.Close(fd)
		assert.Nil(ts.T, err, "Close: %v", err)
		ch <- true
	}(ch)
	fd, err := ts.Open(pipe, sp.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.Close(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Remove(pipe)

	ts.Shutdown()
}

func TestPipeRemove(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	pipe := gopath.Join(pathname, "pipe")

	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		fsl, err := fslib.MakeFsLibAddr("reader", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
		_, err = fsl.Open(pipe, sp.OREAD)
		assert.NotNil(ts.T, err, "Open")
		ch <- true
	}(ch)
	time.Sleep(500 * time.Millisecond)
	err = ts.Remove(pipe)
	assert.Nil(ts.T, err, "Remove")

	<-ch

	ts.Shutdown()
}

func TestPipeCrash0(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	pipe := gopath.Join(pathname, "pipe")
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	go func() {
		fsl, err := fslib.MakeFsLibAddr("writer", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
		_, err = fsl.Open(pipe, sp.OWRITE)
		assert.Nil(ts.T, err, "Open")
		time.Sleep(200 * time.Millisecond)
		// simulate thread crashing
		srv, _, err := ts.PathLastSymlink(pathname)
		assert.Nil(t, err)
		err = fsl.Disconnect(srv)
		assert.Nil(ts.T, err, "Disconnect")

	}()
	fd, err := ts.Open(pipe, sp.OREAD)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Read(fd, 100)
	assert.NotNil(ts.T, err, "read")

	ts.Remove(pipe)
	ts.Shutdown()
}

func TestPipeCrash1(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	pipe := gopath.Join(pathname, "pipe")
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	fsl1, err := fslib.MakeFsLibAddr("w1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(t, err)
	go func() {
		// blocks
		_, err := fsl1.Open(pipe, sp.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	// simulate crash of w1
	srv, _, err := ts.PathLastSymlink(pathname)
	assert.Nil(t, err)
	err = fsl1.Disconnect(srv)
	assert.Nil(ts.T, err, "Disconnect")

	time.Sleep(2 * sp.Conf.Session.TIMEOUT)

	// start up second write to pipe
	go func() {
		fsl2, err := fslib.MakeFsLibAddr("w2", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
		// the pipe has been closed for writing due to crash;
		// this open should fail.
		_, err = fsl2.Open(pipe, sp.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	fd, err := ts.Open(pipe, sp.OREAD)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Read(fd, 100)
	assert.NotNil(ts.T, err, "read")

	ts.Remove(pipe)
	ts.Shutdown()
}
