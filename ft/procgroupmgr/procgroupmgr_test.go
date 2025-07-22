package procgroupmgr_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	//db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/namesrv/ndclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
)

const (
	MCPU            proc.Tmcpu = 1000
	CRASH_SEM_DELAY            = 100 * time.Millisecond
	CRASHFILE                  = "###crashfile##!"
)

func TestCompile(t *testing.T) {
}

func makeNdMgr(ts *test.Tstate) (*ndclnt.NdMgr, error) {
	cfg := procgroupmgr.NewProcGroupConfigRealmSwitch(1, sp.NAMEDREL, nil, 0, test.REALM1.String(), test.REALM1, true)
	ndg, err := ndclnt.NewNdGrpMgr(ts.SigmaClnt, test.REALM1, cfg, true)
	if err != nil {
		return nil, err
	}
	if err := ndg.StartNamedGrp(); err != nil {
		return nil, err
	}
	return ndg, ndg.WaitNamed()
}

func TestStartStop(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	ndg, err := makeNdMgr(ts)
	if !assert.Nil(t, err1, "Error makeNdMgr: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}
	sts, err := sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.True(t, sp.Present(sts, []string{".pstatsd"}), sts)

	err = ndg.StopNamedGrp()
	assert.Nil(t, err)
}

func TestCrashRestart(t *testing.T) {
	const T = 1000
	crashpn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPath(crash.NAMED_CRASH, T, float64(1.0), crashpn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	ndg, err := makeNdMgr(ts)
	if !assert.Nil(t, err1, "Error makeNdMgr: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}

	fn := filepath.Join(sp.NAMED, "fff")
	_, err = sc.PutFile(fn, 0777, sp.OREAD, nil)
	assert.Nil(t, err)

	// Wait for a bit for the crash semaphore to be created
	time.Sleep(CRASH_SEM_DELAY)

	// Tell named to crash
	err = crash.SignalFailer(sc.FsLib, crashpn)
	assert.Nil(t, err, "Err crash: %v", err)

	sts, err := sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.Nil(t, err, "Get named dir post-crash")
	assert.True(t, sp.Present(sts, []string{"fff"}))

	err = ndg.StopNamedGrp()
	assert.Nil(t, err)
}

func TestRecover(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	ndg, err := makeNdMgr(ts)
	if !assert.Nil(t, err1, "Error makeNdMgr: %v", err) {
		return
	}

	ndg.Cfg().Persist(ts.SigmaClnt.FsLib)

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}

	fn := filepath.Join(sp.NAMED, "fff")
	_, err = sc.PutFile(fn, 0777, sp.OREAD, nil)
	assert.Nil(t, err)

	err = ndg.Grp().Crash()
	assert.Nil(t, err)

	err = ndg.StopNamedGrp()
	assert.Nil(t, err)

	time.Sleep(sp.EtcdSessionExpired * time.Second)

	cfgs, err := procgroupmgr.Recover(ts.SigmaClnt)
	assert.Nil(t, err, "Recover")
	assert.Equal(t, 1, len(cfgs))

	pgm := cfgs[0].StartGrpMgr(ts.SigmaClnt)

	ndc, err := ndclnt.NewNdClnt(ts.SigmaClnt, test.REALM1)
	assert.Nil(t, err)

	err = ndc.WaitNamed()
	assert.Nil(t, err)

	sts, err := sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.Nil(t, err, "Get named dir post-recover")
	assert.True(t, sp.Present(sts, []string{"fff"}))

	pgm.StopGroup()
}
