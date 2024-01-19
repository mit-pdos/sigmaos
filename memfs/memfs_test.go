package memfs_test

import (
	"flag"
	gopath "path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/fslibtest"

func init() {
	// use a memfs file system
	flag.StringVar(&pathname, "path", "name/memfs/~local/", "path for file system")
}

func TestCompile(t *testing.T) {
}

type Tstate struct {
	*test.Tstate
	p *proc.Proc
}

func newTstate(t *testing.T, pn string) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.NewTstatePath(t, pathname)
	if pn == gopath.Join(sp.MEMFS, "~local/")+"/" {
		ts.p = proc.NewProc("memfsd", []string{})
		err := ts.Spawn(ts.p)
		assert.Nil(t, err)
	}
	return ts
}

func (ts *Tstate) shutdown() {
	if ts.p != nil {
		err := ts.Evict(ts.p.GetPid())
		assert.Nil(ts.T, err, "evict")
		_, err = ts.WaitExit(ts.p.GetPid())
		assert.Nil(ts.T, err, "WaitExit error")
	}
	ts.Tstate.Shutdown()
}

func TestMemfsd(t *testing.T) {
	ts := newTstate(t, pathname)
	sts, err := ts.GetDir(pathname)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "%v %v\n", pathname, sp.Names(sts))
	ts.shutdown()
}

func TestPipeBasic(t *testing.T) {
	ts := newTstate(t, pathname)

	pipe := gopath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	ch := make(chan bool)
	go func() {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := sigmaclnt.NewFsLib(pcfg)
		assert.Nil(t, err)
		fd, err := fsl.Open(pipe, sp.OREAD)
		assert.Nil(ts.T, err, "Open")
		b, err := fsl.Read(fd, 100)
		assert.Nil(ts.T, err, "Read")
		assert.Equal(ts.T, "hello", string(b))
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

	ts.shutdown()
}

func TestPipeClose(t *testing.T) {
	ts := newTstate(t, pathname)

	pipe := gopath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := sigmaclnt.NewFsLib(pcfg)
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

	ts.shutdown()
}

func TestPipeRemove(t *testing.T) {
	ts := newTstate(t, pathname)
	pipe := gopath.Join(pathname, "pipe")

	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := sigmaclnt.NewFsLib(pcfg)
		assert.Nil(t, err)
		_, err = fsl.Open(pipe, sp.OREAD)
		assert.NotNil(ts.T, err, "Open")
		ch <- true
	}(ch)
	time.Sleep(500 * time.Millisecond)
	err = ts.Remove(pipe)
	assert.Nil(ts.T, err, "Remove")

	<-ch

	ts.shutdown()
}

func TestPipeCrash0(t *testing.T) {
	ts := newTstate(t, pathname)
	pipe := gopath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	go func() {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := sigmaclnt.NewFsLib(pcfg)
		assert.Nil(t, err)
		_, err = fsl.Open(pipe, sp.OWRITE)
		assert.Nil(ts.T, err, "Open")
		time.Sleep(200 * time.Millisecond)
		// simulate fsl crashing
		fsl.Close()
	}()
	fd, err := ts.Open(pipe, sp.OREAD)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Read(fd, 100)
	assert.NotNil(ts.T, err, "read")

	ts.Remove(pipe)
	ts.shutdown()
}

func TestPipeCrash1(t *testing.T) {
	ts := newTstate(t, pathname)
	pipe := gopath.Join(pathname, "pipe")
	err := ts.NewPipe(pipe, 0777)
	assert.Nil(ts.T, err, "NewPipe")

	pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
	fsl1, err := sigmaclnt.NewFsLib(pcfg)

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
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl2, err := sigmaclnt.NewFsLib(pcfg)
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
	ts.shutdown()
}
