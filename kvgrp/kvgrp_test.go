package kvgrp_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/groupmgr"
	"sigmaos/kvgrp"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	GRP    = "grp-0"
	N_REPL = 3
	N_KEYS = 10000
)

type Tstate struct {
	*test.Tstate
	grp string
	gm  *groupmgr.GroupMgr
	job string
}

func newTstate(t1 *test.Tstate, nrepl int, persist bool) *Tstate {
	ts := &Tstate{job: rand.String(4), grp: GRP}
	ts.Tstate = t1
	ts.MkDir(kvgrp.KVDIR, 0777)
	err := ts.MkDir(kvgrp.JobDir(ts.job), 0777)
	assert.Nil(t1.T, err)
	mcfg := groupmgr.NewGroupConfig(nrepl, "kvd", []string{ts.grp, strconv.FormatBool(test.Overlays)}, 0, ts.job)
	if persist {
		mcfg.Persist(ts.SigmaClnt.FsLib)
	}
	ts.gm = mcfg.StartGrpMgr(ts.SigmaClnt, 0)
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(t1.T, err)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	return ts
}

func (ts *Tstate) Shutdown(crash bool) {
	if crash {
		err := ts.gm.Crash()
		assert.Nil(ts.T, err)
	}
	ts.Tstate.Shutdown()
}

func TestCompile(t *testing.T) {
}

func TestStartStopRepl0(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, false)

	sts, _, err := ts.ReadDir(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	_, err = ts.gm.StopGroup()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown(false)
}

func TestStartStopReplN(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, N_REPL, false)
	_, err := ts.gm.StopGroup()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown(false)
}

func (ts *Tstate) testRecover() {
	ts.Shutdown(true)
	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)
	t1, err1 := test.NewTstateAll(ts.T)
	if !assert.Nil(ts.T, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts.Tstate = t1
	gms, err := groupmgr.Recover(ts.SigmaClnt)
	assert.Nil(ts.T, err, "Recover")
	assert.Equal(ts.T, 1, len(gms))
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(ts.T, err)
	time.Sleep(1 * fsetcd.LeaseTTL * time.Second)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	time.Sleep(1 * fsetcd.LeaseTTL * time.Second)
	gms[0].StopGroup()
	ts.RmDir(groupmgr.GRPMGRDIR)
	ts.Shutdown(false)
}

func TestRestartRepl0(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, true)
	ts.testRecover()
}

func TestRestartReplN(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, N_REPL, true)
	ts.testRecover()
}

const (
	CRASH     = 1000
	PARTITION = 200
	NETFAIL   = 200
	NTRIALS   = "3001"
)

type Tstate struct {
	*test.Tstate
	grp string
	gm  *groupmgr.GroupMgr
	job string
}

func newTstate(t1 *test.Tstate, ncrash, crash, partition, netfail int) *Tstate {
	ts := &Tstate{job: rand.String(4), grp: GRP}
	ts.Tstate = t1
	ts.MkDir(kvgrp.KVDIR, 0777)
	err := ts.MkDir(kvgrp.JobDir(ts.job), 0777)
	assert.Nil(t1.T, err)
	mcfg := groupmgr.NewGroupConfig(0, "kvd", []string{ts.grp, strconv.FormatBool(test.Overlays)}, 0, ts.job)
	mcfg.SetTest(crash, partition, netfail)
	ts.gm = mcfg.StartGrpMgr(ts.SigmaClnt, ncrash)
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(t1.T, err)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	return ts
}

func TestCompile(t *testing.T) {
}

// Server crashes storing a semaphore. The test's down() will return a
// not-found for the semaphore, which is interpreted as a successful
// down by the semclnt.
func TestServerCrash(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 1, CRASH, 0, 0)

	sem := semclnt.NewSemClnt(ts.FsLib, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
	err := sem.Init(0)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Sem %v", kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")

	ch := make(chan error)
	go func() {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := sigmaclnt.NewFsLib(pcfg)
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
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
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
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
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
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
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
	const N = 10
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 0, 0, NETFAIL)

	ch := make(chan error)
	go func() {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
		fsl, err := sigmaclnt.NewFsLib(pcfg)
		assert.Nil(t, err)
		for i := 0; i < N; i++ {
			_, err := fsl.Stat(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
			if err != nil {
				ch <- err
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		ch <- nil
	}()

	err := <-ch
	assert.Nil(ts.T, err, "fsl1")

	ts.gm.StopGroup()
	ts.Shutdown()
}

func (ts *Tstate) stat(t *testing.T, i int, ch chan error) {
	pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
	fsl, err := sigmaclnt.NewFsLib(pcfg)
	assert.Nil(t, err)
	for true {
		_, err := fsl.Stat(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
		if err != nil {
			db.DPrintf(db.TEST, "Stat %d err %v", i, err)
			ch <- err
			break
		}
	}
	db.DPrintf(db.TEST, "Client %v %v done", fsl.ClntId(), i)
	fsl.Close()
}

func TestServerPartitionNonBlockingSimple(t *testing.T) {
	const N = 3

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 0, PARTITION, 0)
	ch := make(chan error)
	for i := 0; i < N; i++ {
		go ts.stat(t, i, ch)
		err := <-ch
		assert.NotNil(ts.T, err, "stat")
	}
	db.DPrintf(db.TEST, "Stopping group")
	ts.gm.StopGroup()
	ts.Shutdown()
}

func TestServerPartitionNonBlockingConcur(t *testing.T) {
	const N = sessstatesrv.NLAST

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 0, PARTITION, 0)
	ch := make(chan error)
	for i := 0; i < N; i++ {
		go ts.stat(t, i, ch)
	}
	for i := 0; i < N; i++ {
		err := <-ch
		assert.NotNil(ts.T, err, "stat")
	}
	db.DPrintf(db.TEST, "Stopping group")
	ts.gm.StopGroup()
	ts.Shutdown()
}

func TestServerPartitionBlocking(t *testing.T) {
	const N = 10

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 0, PARTITION, 0)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func(i int) {
			pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
			fsl, err := sigmaclnt.NewFsLib(pcfg)
			assert.Nil(t, err)
			sem := semclnt.NewSemClnt(fsl, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
			sem.Init(0)
			err = sem.Down()
			ch <- err
			fsl.Close()
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
	fsl, err := sigmaclnt.NewFsLib(pcfg)
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

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
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
