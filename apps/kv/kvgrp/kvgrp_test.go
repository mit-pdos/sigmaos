package kvgrp_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/kv/kvgrp"
	db "sigmaos/debug"
	"sigmaos/groupmgr"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	GRP       = "grp-0"
	N_REPL    = 3
	CRASH     = 1000
	PARTITION = 200
	NETFAIL   = 200
)

type Tstate struct {
	*test.Tstate
	grp string
	gm  *groupmgr.GroupMgr
	job string
}

func newTstate(t1 *test.Tstate, nrepl, ncrash, crash, partition, netfail int, persist bool) *Tstate {
	ts := &Tstate{job: rand.String(4), grp: GRP}
	ts.Tstate = t1
	ts.MkDir(kvgrp.KVDIR, 0777)
	err := ts.MkDir(kvgrp.JobDir(ts.job), 0777)
	assert.Nil(t1.T, err)
	mcfg := groupmgr.NewGroupConfig(nrepl, "kvd", []string{ts.grp, strconv.FormatBool(test.Overlays)}, 0, ts.job)
	if persist {
		mcfg.Persist(ts.SigmaClnt.FsLib)
	}
	mcfg.SetTest(crash, partition, netfail)
	ts.gm = mcfg.StartGrpMgr(ts.SigmaClnt, ncrash)
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
	ts := newTstate(t1, 0, 0, 0, 0, 0, false)

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
	ts := newTstate(t1, N_REPL, 0, 0, 0, 0, false)
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
	ts := newTstate(t1, 0, 0, 0, 0, 0, true)
	ts.testRecover()
}

func TestRestartReplN(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, N_REPL, 0, 0, 0, 0, true)
	ts.testRecover()
}

// kvd crashes storing a semaphore. The test's down() will return a
// not-found for the semaphore, which is interpreted as a successful
// down by the semclnt.
func TestServerCrash(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 1, CRASH, 0, 0, false)

	sem := semclnt.NewSemClnt(ts.FsLib, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
	err := sem.Init(0)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Sem %v", kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")

	ch := make(chan error)
	go func() {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
		assert.Nil(t, err)
		sem := semclnt.NewSemClnt(fsl, kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp)+"/sem")
		err = sem.Down()
		ch <- err
	}()

	err = <-ch
	assert.Nil(ts.T, err, "down")

	ts.gm.StopGroup()

	ts.Shutdown(false)
}

func TestReconnectSimple(t *testing.T) {
	const N = 10
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 0, 0, 0, NETFAIL, false)

	ch := make(chan error)
	go func() {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
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
	ts.Shutdown(false)
}

func (ts *Tstate) stat(t *testing.T, i int, ch chan error) {
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
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
	ts := newTstate(t1, 0, 0, 0, PARTITION, 0, false)
	ch := make(chan error)
	for i := 0; i < N; i++ {
		go ts.stat(t, i, ch)
		err := <-ch
		assert.NotNil(ts.T, err, "stat")
	}
	db.DPrintf(db.TEST, "Stopping group")
	ts.gm.StopGroup()
	ts.Shutdown(false)
}

func TestServerPartitionNonBlockingConcur(t *testing.T) {
	const N = sesssrv.NLAST

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 0, 0, PARTITION, 0, false)
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
	ts.Shutdown(false)
}

func TestServerPartitionBlocking(t *testing.T) {
	const N = 10

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, 0, 0, 0, PARTITION, 0, false)

	for i := 0; i < N; i++ {
		ch := make(chan error)
		go func(i int) {
			pe := proc.NewAddedProcEnv(ts.ProcEnv())
			fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
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
	ts.Shutdown(false)
}
