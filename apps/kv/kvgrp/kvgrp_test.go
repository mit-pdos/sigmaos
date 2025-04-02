package kvgrp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/kv"
	"sigmaos/apps/kv/kvgrp"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	sesssrv "sigmaos/session/srv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/coordination/semaphore"
	"sigmaos/util/crash"
	"sigmaos/util/rand"
)

const (
	GRP       = "grp-0"
	PARTITION = 200
	NETFAIL   = 200
)

var EvP = crash.NewEvent(crash.KVD_PARTITION, PARTITION, 0.33)

type Tstate struct {
	mrts *test.MultiRealmTstate
	grp  string
	gm   *procgroupmgr.ProcGroupMgr
	job  string
}

func newTstate(mrts *test.MultiRealmTstate, nrepl int, persist bool) *Tstate {
	ts := &Tstate{
		job:  rand.Name(),
		grp:  GRP,
		mrts: mrts,
	}
	ts.mrts.GetRealm(test.REALM1).MkDir(kvgrp.KVDIR, 0777)
	err := ts.mrts.GetRealm(test.REALM1).MkDir(kvgrp.JobDir(ts.job), 0777)
	assert.Nil(mrts.T, err)
	mcfg := procgroupmgr.NewProcGroupConfig(nrepl, "kvd", []string{ts.grp}, 0, ts.job)
	if persist {
		mcfg.Persist(ts.mrts.GetRealm(test.REALM1).SigmaClnt.FsLib)
	}
	ts.gm = mcfg.StartGrpMgr(ts.mrts.GetRealm(test.REALM1).SigmaClnt)
	cfg, err := kvgrp.WaitStarted(ts.mrts.GetRealm(test.REALM1).SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(mrts.T, err)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	return ts
}

func (ts *Tstate) Shutdown(crash bool, reboot bool) {
	if crash {
		err := ts.gm.Crash()
		assert.Nil(ts.mrts.T, err)
	}
	if reboot {
		ts.mrts.ShutdownForReboot()
	} else {
		ts.mrts.Shutdown()
	}
}

func TestCompile(t *testing.T) {
}

func TestStartStopRepl0(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(mrts, kv.KVD_NO_REPL, false)

	sts, _, err := ts.mrts.GetRealm(test.REALM1).ReadDir(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	_, err = ts.mrts.GetRealm(test.REALM1).GetFile(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp))
	assert.Nil(t, err, "GetFile")

	_, err = ts.gm.StopGroup()
	assert.Nil(ts.mrts.T, err, "Stop")
	ts.Shutdown(false, false)
}

func TestStartStopReplN(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(mrts, kv.KVD_REPL_LEVEL, false)

	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	_, err := ts.mrts.GetRealm(test.REALM1).GetFile(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp))
	assert.Nil(t, err, "GetFile")

	_, err = ts.gm.StopGroup()
	assert.Nil(ts.mrts.T, err, "Stop")
	ts.Shutdown(false, false)
}

// XXX TODO shutdown?
func (ts *Tstate) testRecover() {
	ts.Shutdown(true, true)
	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)
	mrts, err1 := test.NewMultiRealmTstate(ts.mrts.T, []sp.Trealm{test.REALM1})
	if !assert.Nil(ts.mrts.T, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts.mrts = mrts
	gms, err := procgroupmgr.Recover(ts.mrts.GetRealm(test.REALM1).SigmaClnt)
	assert.Nil(ts.mrts.T, err, "Recover")
	assert.Equal(ts.mrts.T, 1, len(gms))
	cfg, err := kvgrp.WaitStarted(ts.mrts.GetRealm(test.REALM1).SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(ts.mrts.T, err)
	time.Sleep(1 * fsetcd.LeaseTTL * time.Second)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	time.Sleep(1 * fsetcd.LeaseTTL * time.Second)
	gms[0].StopGroup()
	ts.mrts.GetRealm(test.REALM1).RmDir(procgroupmgr.GRPMGRDIR)
	ts.Shutdown(false, false)
}

func TestRestartRepl0(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(mrts, kv.KVD_NO_REPL, true)
	ts.testRecover()
}

func TestRestartReplN(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(mrts, kv.KVD_REPL_LEVEL, true)
	ts.testRecover()
}

// kvd crashes storing a semaphore. The test's down() will return a
// not-found for the semaphore, which is interpreted as a successful
// down by the semaphore.
func TestServerCrash(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	e0 := crash.NewEvent(crash.KVD_CRASH, kvgrp.CRASH, 0.33)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e0))

	ts := newTstate(mrts, kv.KVD_NO_REPL, false)

	sem := semaphore.NewSemaphore(ts.mrts.GetRealm(test.REALM1).FsLib, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
	err = sem.Init(0)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Sem %v", kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")

	ch := make(chan error)
	go func() {
		pe := proc.NewAddedProcEnv(ts.mrts.GetRealm(test.REALM1).ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
		assert.Nil(t, err)
		sem := semaphore.NewSemaphore(fsl, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
		err = sem.Down()
		ch <- err
	}()

	err = <-ch
	assert.Nil(ts.mrts.T, err, "down")

	ts.gm.StopGroup()

	ts.Shutdown(false, false)
}

func TestReconnectSimple(t *testing.T) {
	const N = 100
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	e0 := crash.NewEvent(crash.KVD_NETFAIL, NETFAIL, 0.33)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e0))

	ts := newTstate(mrts, kv.KVD_NO_REPL, false)

	ch := make(chan error)
	go func() {
		pe := proc.NewAddedProcEnv(ts.mrts.GetRealm(test.REALM1).ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
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
	err = <-ch
	assert.Nil(ts.mrts.T, err, "fsl1")
	ts.gm.StopGroup()
	ts.Shutdown(false, false)
}

func (ts *Tstate) stat(t *testing.T, i int, ch chan error) {
	pe := proc.NewAddedProcEnv(ts.mrts.GetRealm(test.REALM1).ProcEnv())
	fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
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

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err := crash.SetSigmaFail(crash.NewTeventMapOne(EvP))
	assert.Nil(t, err)
	ts := newTstate(mrts, kv.KVD_NO_REPL, false)

	ch := make(chan error)
	for i := 0; i < N; i++ {
		go ts.stat(t, i, ch)
		err := <-ch
		assert.NotNil(ts.mrts.T, err, "stat")
	}
	db.DPrintf(db.TEST, "Stopping group")
	ts.gm.StopGroup()
	ts.Shutdown(false, false)
}

func TestServerPartitionNonBlockingConcur(t *testing.T) {
	const N = sesssrv.NLAST

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err := crash.SetSigmaFail(crash.NewTeventMapOne(EvP))
	assert.Nil(t, err)
	ts := newTstate(mrts, kv.KVD_NO_REPL, false)
	ch := make(chan error)
	for i := 0; i < N; i++ {
		go ts.stat(t, i, ch)
	}
	for i := 0; i < N; i++ {
		err := <-ch
		assert.NotNil(ts.mrts.T, err, "stat")
	}
	db.DPrintf(db.TEST, "Stopping group")
	ts.gm.StopGroup()
	ts.Shutdown(false, false)
}

func TestServerPartitionBlocking(t *testing.T) {
	const N = 10

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err := crash.SetSigmaFail(crash.NewTeventMapOne(EvP))
	assert.Nil(t, err)

	ts := newTstate(mrts, kv.KVD_NO_REPL, false)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func(i int) {
			pe := proc.NewAddedProcEnv(ts.mrts.GetRealm(test.REALM1).ProcEnv())
			fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
			assert.Nil(t, err)
			sem := semaphore.NewSemaphore(fsl, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
			sem.Init(0)
			err = sem.Down()
			ch <- err
			fsl.Close()
		}(i)

		err := <-ch
		assert.NotNil(ts.mrts.T, err, "down")
	}
	ts.gm.StopGroup()
	ts.Shutdown(false, false)
}
