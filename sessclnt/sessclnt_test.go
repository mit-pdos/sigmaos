package sessclnt_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/groupmgr"
	"sigmaos/kvgrp"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	CRASH     = 1000
	PARTITION = 200
	NETFAIL   = 200
	NTRIALS   = "3001"
	GRP       = "grp-0"
)

type Tstate struct {
	*test.Tstate
	grp string
	gm  *groupmgr.GroupMgr
	job string
}

func newTstate(t *testing.T, ncrash, crash, partition, netfail int) *Tstate {
	ts := &Tstate{job: rand.String(4), grp: GRP}
	ts.Tstate = test.NewTstateAll(t)
	ts.MkDir(kvgrp.KVDIR, 0777)
	err := ts.MkDir(kvgrp.JobDir(ts.job), 0777)
	assert.Nil(t, err)
	mcfg := groupmgr.NewGroupConfig(0, "kvd", []string{ts.grp, strconv.FormatBool(test.Overlays)}, 0, ts.job)
	mcfg.SetTest(crash, partition, netfail)
	ts.gm = mcfg.StartGrpMgr(ts.SigmaClnt, ncrash)
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	return ts
}

// Server crashes storing a semaphore. The test's down() will return a
// not-found for the semaphore, which is interpreted as a successful
// down by the semclnt.
func TestServerCrash(t *testing.T) {
	ts := newTstate(t, 1, CRASH, 0, 0)

	sem := semclnt.NewSemClnt(ts.FsLib, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
	err := sem.Init(0)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Sem %v", kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")

	ch := make(chan error)
	go func() {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := fslib.NewFsLib(pcfg)
		assert.Nil(t, err)
		sem := semclnt.NewSemClnt(fsl, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
		err = sem.Down()
		ch <- err
	}()

	err = <-ch
	assert.Nil(ts.T, err, "down")

	ts.gm.StopGroup()

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
	ts := test.NewTstateAll(t)
	a := proc.NewProc("proctest", []string{NTRIALS, "sleeper", "1us", ""})
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
	ts := test.NewTstateAll(t)
	a := proc.NewProc("proctest", []string{NTRIALS, "crash"})
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
	ts := test.NewTstateAll(t)
	a := proc.NewProc("proctest", []string{NTRIALS, "partition"})
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
	ts := newTstate(t, 0, 0, 0, NETFAIL)

	ch := make(chan error)
	go func() {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := fslib.NewFsLib(pcfg)
		assert.Nil(t, err)
		for i := 0; i < N; i++ {
			_, err := fsl.Stat(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
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

	ts.gm.StopGroup()
	ts.Shutdown()
}

func TestServerPartitionNonBlocking(t *testing.T) {
	const N = 50

	ts := newTstate(t, 0, 0, PARTITION, 0)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func(i int) {
			pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
			fsl, err := fslib.NewFsLib(pcfg)
			assert.Nil(t, err)
			for true {
				_, err := fsl.Stat(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
				if err != nil {
					ch <- err
					break
				}
			}
			db.DPrintf(db.TEST, "Client %v done", i)
			fsl.DetachAll()
		}(i)

		err := <-ch
		assert.NotNil(ts.T, err, "stat")
	}
	db.DPrintf(db.TEST, "Stopping group")
	ts.gm.StopGroup()
	ts.Shutdown()
}

func TestServerPartitionBlocking(t *testing.T) {
	const N = 50

	ts := newTstate(t, 0, 0, PARTITION, 0)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func(i int) {
			pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
			fsl, err := fslib.NewFsLib(pcfg)
			assert.Nil(t, err)
			sem := semclnt.NewSemClnt(fsl, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
			sem.Init(0)
			err = sem.Down()
			ch <- err
			fsl.DetachAll()
		}(i)

		err := <-ch
		assert.NotNil(ts.T, err, "down")
	}
	ts.gm.StopGroup()
	ts.Shutdown()
}

const (
	FILESZ  = 50 * sp.MBYTE
	WRITESZ = 4096
)

func writer(t *testing.T, ch chan error, pcfg *proc.ProcEnv) {
	fsl, err := fslib.NewFsLib(pcfg)
	assert.Nil(t, err)
	fn := sp.UX + "~local/file-" + string(pcfg.GetUname())
	stop := false
	nfile := 0
	for !stop {
		select {
		case <-ch:
			stop = true
		default:
			if err := fsl.Remove(fn); serr.IsErrCode(err, serr.TErrUnreachable) {
				break
			}
			w, err := fsl.CreateAsyncWriter(fn, 0777, sp.OWRITE)
			if err != nil {
				assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))
				break
			}
			nfile += 1
			buf := test.NewBuf(WRITESZ)
			if err := test.Writer(t, w, buf, FILESZ); err != nil {
				break
			}
			if err := w.Close(); err != nil {
				assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))
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

	ts := test.NewTstateAll(t)
	ch := make(chan error)

	for i := 0; i < N; i++ {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
		go writer(ts.T, ch, pcfg)
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
