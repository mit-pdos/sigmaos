package sessclnt_test

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/group"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	GRP0      = "GRP0"
	CRASH     = 1000
	PARTITION = 200
	NETFAIL   = 200
	NTRIALS   = "3001"
	JOBDIR    = "name/"
)

var DIRGRP0 = group.GrpPath(JOBDIR, GRP0) + "/"

func TestWaitClosed(t *testing.T) {
	ts := test.MakeTstateAll(t)

	grp := groupmgr.Start(ts.SigmaClnt, 0, "kvd", []string{GRP0, strconv.FormatBool(test.Overlays)}, JOBDIR, 0, 1, CRASH, 0, 0)
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

	grp := groupmgr.Start(ts.SigmaClnt, 0, "kvd", []string{GRP0, strconv.FormatBool(test.Overlays)}, JOBDIR, 0, 1, CRASH, 0, 0)

	sem := semclnt.MakeSemClnt(ts.FsLib, DIRGRP0+"sem")
	err := sem.Init(0)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Sem %v", DIRGRP0+"sem")

	ch := make(chan error)
	go func() {
		fsl, err := fslib.MakeFsLibAddr("fslibtest-1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
		sem := semclnt.MakeSemClnt(fsl, DIRGRP0+"sem")
		err = sem.Down()
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
	a := proc.MakeProc("proctest", []string{NTRIALS, "sleeper", "1us", ""})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	ts.Shutdown()
}

func TestProcCrashMany(t *testing.T) {
	ts := test.MakeTstateAll(t)
	a := proc.MakeProc("proctest", []string{NTRIALS, "crash"})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	ts.Shutdown()
}

func TestProcPartitionMany(t *testing.T) {
	ts := test.MakeTstateAll(t)
	a := proc.MakeProc("proctest", []string{NTRIALS, "partition"})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	if assert.NotNil(t, status, "nil status") {
		assert.True(t, status.IsStatusOK(), status)
	}
	ts.Shutdown()
}

func TestReconnectSimple(t *testing.T) {
	const N = 1000
	ts := test.MakeTstateAll(t)
	grp := groupmgr.Start(ts.SigmaClnt, 0, "kvd", []string{GRP0, strconv.FormatBool(test.Overlays)}, JOBDIR, 0, 0, 0, 0, NETFAIL)
	ch := make(chan error)
	go func() {
		fsl, err := fslib.MakeFsLibAddr("fslibtest-1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
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
	grp := groupmgr.Start(ts.SigmaClnt, 0, "kvd", []string{GRP0, strconv.FormatBool(test.Overlays)}, JOBDIR, 0, 0, 0, PARTITION, 0)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func(i int) {
			fsl, err := fslib.MakeFsLibAddr(fmt.Sprintf("test-fsl-%v", i), sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
			assert.Nil(t, err)
			for true {
				_, err := fsl.Stat(DIRGRP0)
				if err != nil {
					ch <- err
					break
				}
			}
			db.DPrintf(db.TEST, "Client %v done", i)
			fsl.Exit()
		}(i)

		err := <-ch
		assert.NotNil(ts.T, err, "stat")
	}
	db.DPrintf(db.TEST, "Stopping group")
	grp.Stop()
	ts.Shutdown()
}

func TestServerPartitionBlocking(t *testing.T) {
	const N = 50

	ts := test.MakeTstateAll(t)
	grp := groupmgr.Start(ts.SigmaClnt, 0, "kvd", []string{GRP0, strconv.FormatBool(test.Overlays)}, JOBDIR, 0, 0, 0, PARTITION, 0)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func() {
			fsl, err := fslib.MakeFsLibAddr("fsl", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
			assert.Nil(t, err)
			sem := semclnt.MakeSemClnt(fsl, DIRGRP0+"sem")
			sem.Init(0)
			err = sem.Down()
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
	FILESZ  = 50 * sp.MBYTE
	WRITESZ = 4096
)

func writer(t *testing.T, ch chan error, name, lip string, nds sp.Taddrs) {
	fsl, err := fslib.MakeFsLibAddr("writer-"+name, sp.ROOTREALM, lip, nds)
	assert.Nil(t, err)
	fn := sp.UX + "~local/file-" + name
	stop := false
	nfile := 0
	for !stop {
		select {
		case <-ch:
			stop = true
		default:
			var serr *serr.Err
			if err := fsl.Remove(fn); errors.As(err, &serr) && serr.IsErrUnreachable() {
				break
			}
			w, err := fsl.CreateAsyncWriter(fn, 0777, sp.OWRITE)
			if errors.As(err, &serr) {
				assert.True(t, serr.IsErrUnreachable())
				break
			}
			nfile += 1
			buf := test.MkBuf(WRITESZ)
			if err := test.Writer(t, w, buf, FILESZ); err != nil {
				break
			}
			if err := w.Close(); errors.As(err, &serr) {
				assert.True(t, serr.IsErrUnreachable())
				break
			}
		}
	}
	assert.True(t, nfile >= 3) // a bit arbitrary
	fsl.Remove(fn)
}

func TestWriteCrash(t *testing.T) {
	const (
		N        = 20
		NCRASH   = 5
		CRASHSRV = 1000000
	)

	ts := test.MakeTstateAll(t)
	ch := make(chan error)

	for i := 0; i < N; i++ {
		go writer(ts.T, ch, strconv.Itoa(i), ts.GetLocalIP(), ts.NamedAddr())
	}

	crashchan := make(chan bool)
	l := &sync.Mutex{}
	for i := 0; i < NCRASH; i++ {
		go ts.CrashServer(sp.UXREL, (i+1)*CRASHSRV, l, crashchan)
	}

	for i := 0; i < NCRASH; i++ {
		<-crashchan
	}

	for i := 0; i < N; i++ {
		ch <- nil
	}

	ts.Shutdown()
}
