package memfs_test

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/sp.ANY/fslibtest"

func init() {
	// use a memfs file system
	flag.StringVar(&pathname, "path", filepath.Join("name/memfs", sp.ANY)+"/", "path for file system")
}

func TestCompile(t *testing.T) {
}

func TestMemfsd(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	sts, err := ts.GetDir(pathname)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "%v %v\n", pathname, sp.Names(sts))
	ts.Shutdown()
}

func TestPipeBasic(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pipe := filepath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	ch := make(chan bool)
	go func() {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
		assert.Nil(t, err)
		fd, err := fsl.Open(pipe, sp.OREAD)
		assert.Nil(ts.T, err, "Open")
		b := make([]byte, 100)
		n, err := fsl.Read(fd, b)
		assert.Nil(ts.T, err, "Read")
		assert.Equal(ts.T, sp.Tsize(len("hello")), n)
		assert.Equal(ts.T, "hello", string(b[:n]))
		err = fsl.CloseFd(fd)
		assert.Nil(ts.T, err, "Close")
		ch <- true
	}()
	fd, err := ts.Open(pipe, sp.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.CloseFd(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Remove(pipe)

	ts.Shutdown()
}

func TestPipeClose(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pipe := filepath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
		assert.Nil(t, err)
		fd, err := fsl.Open(pipe, sp.OREAD)
		assert.Nil(ts.T, err, "Open")
		for true {
			b := make([]byte, 100)
			n, err := fsl.Read(fd, b)
			if err != nil { // writer closed pipe
				break
			}
			assert.Nil(ts.T, err, "Read")
			assert.Equal(ts.T, "hello", string(b[:n]))
		}
		err = fsl.CloseFd(fd)
		assert.Nil(ts.T, err, "Close: %v", err)
		ch <- true
	}(ch)
	fd, err := ts.Open(pipe, sp.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.CloseFd(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Remove(pipe)

	ts.Shutdown()
}

func TestPipeRemove(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	pipe := filepath.Join(pathname, "pipe")

	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	pipe := filepath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	go func() {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
		assert.Nil(t, err)
		_, err = fsl.Open(pipe, sp.OWRITE)
		assert.Nil(ts.T, err, "Open")
		time.Sleep(200 * time.Millisecond)
		// simulate fsl crashing
		fsl.Close()
	}()
	fd, err := ts.Open(pipe, sp.OREAD)
	assert.Nil(ts.T, err, "Open")
	b := make([]byte, 100)
	_, err = ts.Read(fd, b)
	assert.NotNil(ts.T, err, "read")

	ts.Remove(pipe)
	ts.Shutdown()
}

func TestPipeCrash1(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	pipe := filepath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl1, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))

	assert.Nil(t, err)
	go func() {
		// blocks
		_, err := fsl1.Open(pipe, sp.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	// simulate crash of w1
	fsl1.Close()

	time.Sleep(2 * sp.Conf.Session.TIMEOUT)

	// start up second write to pipe
	go func() {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl2, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
		assert.Nil(t, err)
		// the pipe has been closed for writing due to crash;
		// this open should fail.
		_, err = fsl2.Open(pipe, sp.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	fd, err := ts.Open(pipe, sp.OREAD)
	assert.Nil(ts.T, err, "Open")
	b := make([]byte, 100)
	_, err = ts.Read(fd, b)
	assert.NotNil(ts.T, err, "read")

	ts.Remove(pipe)
	ts.Shutdown()
}
