package sessclnt_test

import (
	"bufio"
	"log"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/awriter"
	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/semclnt"
	"ulambda/test"
)

const (
	GRP0      = "GRP0"
	DIRGRP0   = group.GRPDIR + GRP0 + "/"
	CRASH     = 1000
	PARTITION = 200
	NETFAIL   = 200
	NTRIALS   = "3001"
)

func TestWaitClosed(t *testing.T) {
	ts := test.MakeTstateAll(t)

	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 1, CRASH, 0, 0)
	_, err := ts.Stat(DIRGRP0)
	assert.Nil(t, err, "stat")

	grp.Stop()

	// XXX should hang around until client closes sessions; once WaitClosed() is
	// propertly implemented.

	ts.Detach(DIRGRP0)
	ts.Shutdown()
}

func TestServerCrash(t *testing.T) {
	ts := test.MakeTstateAll(t)

	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 1, CRASH, 0, 0)

	sem := semclnt.MakeSemClnt(ts.FsLib, DIRGRP0+"sem")
	err := sem.Init(0)
	assert.Nil(t, err)

	ch := make(chan error)
	go func() {
		fsl := fslib.MakeFsLibAddr("fslibtest-1", fslib.Named())
		sem := semclnt.MakeSemClnt(fsl, DIRGRP0+"sem")
		err := sem.Down()
		ch <- err
	}()

	err = <-ch
	assert.NotNil(ts.T, err, "down")

	grp.Stop()

	ts.Shutdown()
}

func BurstProc(n int, f func(chan error)) error {
	ch := make(chan error)
	for i := 0; i < n; i++ {
		go f(ch)
	}
	var err error
	for i := 0; i < n; i++ {
		r := <-ch
		if r != nil && err != nil {
			err = r
		}
	}
	return err
}

func TestProcManyOK(t *testing.T) {
	ts := test.MakeTstateAll(t)
	a := proc.MakeProc("bin/user/proctest", []string{NTRIALS, "bin/user/sleeper", "1us", ""})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.Pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.Pid)
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	ts.Shutdown()
}

func TestProcCrashMany(t *testing.T) {
	ts := test.MakeTstateAll(t)
	a := proc.MakeProc("bin/user/proctest", []string{NTRIALS, "bin/user/crash"})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.Pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.Pid)
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	ts.Shutdown()
}

func TestProcPartitionMany(t *testing.T) {
	ts := test.MakeTstateAll(t)
	a := proc.MakeProc("bin/user/proctest", []string{NTRIALS, "bin/user/partition"})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.Pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.Pid)
	assert.Nil(t, err, "waitexit")
	if assert.NotNil(t, status, "nil status") {
		assert.True(t, status.IsStatusOK(), status)
	}
	ts.Shutdown()
}

func TestReconnectSimple(t *testing.T) {
	const N = 1000
	ts := test.MakeTstateAll(t)
	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 0, 0, 0, NETFAIL)
	ch := make(chan error)
	go func() {
		fsl := fslib.MakeFsLibAddr("fslibtest-1", fslib.Named())
		for i := 0; i < N; i++ {
			_, err := fsl.Stat(DIRGRP0)
			if err != nil {
				ch <- err
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		ch <- nil
	}()

	err := <-ch
	assert.Nil(ts.T, err, "fsl1")

	grp.Stop()
	ts.Shutdown()
}

func TestServerPartitionNonBlocking(t *testing.T) {
	const N = 50

	ts := test.MakeTstateAll(t)
	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 0, 0, PARTITION, 0)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func() {
			fsl := fslib.MakeFsLibAddr("fsl", fslib.Named())
			for true {
				_, err := fsl.Stat(DIRGRP0)
				if err != nil {
					ch <- err
					break
				}
			}
			fsl.Exit()
		}()

		err := <-ch
		assert.NotNil(ts.T, err, "stat")
	}
	grp.Stop()
	ts.Shutdown()
}

func TestServerPartitionBlocking(t *testing.T) {
	const N = 50

	ts := test.MakeTstateAll(t)
	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 0, 0, PARTITION, 0)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func() {
			fsl := fslib.MakeFsLibAddr("fsl", fslib.Named())
			sem := semclnt.MakeSemClnt(fsl, DIRGRP0+"sem")
			sem.Init(0)
			err := sem.Down()
			ch <- err
			fsl.Exit()

		}()

		err := <-ch
		assert.NotNil(ts.T, err, "down")
	}
	grp.Stop()
	ts.Shutdown()
}

const (
	MBYTE   = 1 << 20
	FILESZ  = 50 * MBYTE
	WRITESZ = 4096
	BUFSZ   = 1 << 16
)

func writer(t *testing.T, ch chan error, name string) {
	fsl := fslib.MakeFsLibAddr("writer-"+name, fslib.Named())
	fn := np.UX + "~ip/file-" + name
	stop := false
	for !stop {
		select {
		case <-ch:
			log.Printf("done %v\n", name)
			stop = true
		default:
			if err := fsl.Remove(fn); err != nil && np.IsErrUnreachable(err) {
				break
			}
			w, err := fsl.CreateWriter(fn, 0777, np.OWRITE)
			if err != nil {
				log.Printf("create failed %v\n", err)
				assert.True(t, np.IsErrUnreachable(err))
				break
			}
			aw := awriter.NewWriterSize(w, BUFSZ)
			bw := bufio.NewWriterSize(aw, BUFSZ)
			buf := test.MkBuf(WRITESZ)
			if err := test.Writer(t, bw, buf, FILESZ); err != nil {
				log.Printf("write failed %v\n", err)
				break
			}
			if err := bw.Flush(); err != nil {
				log.Printf("flush failed %v\n", err)
				assert.True(t, np.IsErrUnreachable(err))
				break
			}
			if err := aw.Close(); err != nil {
				log.Printf("close failed %v\n", err)
				assert.True(t, np.IsErrUnreachable(err))
			}
		}
	}
	fsl.Remove(fn)
}

func TestWriteCrash(t *testing.T) {
	const (
		N        = 10
		NCRASH   = 5
		CRASHSRV = 1000000
	)

	ts := test.MakeTstateAll(t)
	ch := make(chan error)

	for i := 0; i < N; i++ {
		go writer(ts.T, ch, strconv.Itoa(i))
	}

	crashchan := make(chan bool)
	l := &sync.Mutex{}
	for i := 0; i < NCRASH; i++ {
		go ts.CrashServer(np.UX, (i+1)*CRASHSRV, l, crashchan)
	}

	for i := 0; i < NCRASH; i++ {
		<-crashchan
	}

	for i := 0; i < N; i++ {
		ch <- nil
	}

	ts.Shutdown()
}
