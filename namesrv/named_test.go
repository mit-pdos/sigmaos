package namesrv_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
)

const (
	MCPU            proc.Tmcpu    = 1000
	DELAY           time.Duration = 2 * fsetcd.LeaseTTL * time.Second
	CRASH_SEM_DELAY               = 5 * time.Second
)

func TestCompile(t *testing.T) {
}

func TestBootNamed(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts, err := ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "named %v", sp.Names(sts))

	assert.True(t, sp.Present(sts, namesrv.InitRootDir), "initfs")

	ts.Shutdown()
}

func TestPstats(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	st, err := ts.ReadPstats()
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "pstats %v", st)

	ts.Shutdown()
}

func newNamedProc(mcpu proc.Tmcpu, realm sp.Trealm, dialproxy bool, canFail bool) *proc.Proc {
	p := proc.NewProc(sp.NAMEDREL, []string{realm.String()})
	p.SetMcpu(mcpu)
	p.SetRealmSwitch(realm)
	p.GetProcEnv().UseDialProxy = dialproxy
	if !canFail {
		p.AppendEnv("SIGMAFAIL", "")
	} else {
		p.AppendEnv(proc.SIGMAFAIL, proc.GetSigmaFail())
	}
	return p
}

func startNamed(ts *test.Tstate, nd *proc.Proc) error {
	// Spawn the named proc
	if err := ts.Spawn(nd); !assert.Nil(ts.T, err, "Err spawn named: %v", err) {
		return err
	}
	db.DPrintf(db.TEST, "New named spawned: %v", nd.GetPid())

	// Wait for the proc to start
	if err := ts.WaitStart(nd.GetPid()); !assert.Nil(ts.T, err, "Err WaitStart named: %v", err) {
		return err
	}
	db.DPrintf(db.TEST, "New named started")

	// Wait for the named to start up
	_, err := ts.GetFileWatch(filepath.Join(sp.REALMS, test.REALM1.String()))
	// wait until the named has registered its endpoint and is ready to serve
	if !assert.Nil(ts.T, err, "Err GetFileWatch: %v", err) {
		return err
	}
	db.DPrintf(db.TEST, "New named ready to serve")
	return nil
}

func stopNamed(ts *test.Tstate, nd *proc.Proc) error {
	// Evict the new named
	err := ts.Evict(nd.GetPid())
	assert.Nil(ts.T, err, "Err evict named: %v", err)
	status, err := ts.WaitExit(nd.GetPid())
	if assert.Nil(ts.T, err, "Err WaitExit named: %v", err) {
		assert.True(ts.T, status.IsStatusEvicted(), "Wrong exit status: %v", status)
	}
	// Make sure the named EP has been removed
	ts.Remove(filepath.Join(sp.REALMS, test.REALM1.String()))
	return err
}

func TestKillNamed(t *testing.T) {
	const T = 1000
	fn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPath(crash.NAMED_CRASH, T, float64(1.0), fn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	nd1 := newNamedProc(MCPU, test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := startNamed(ts, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Made new realm sigmaclnt")

	sts, err := sc.GetDir(sp.NAMED + "/")
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "New realm sigmaclnt contents: %v", sp.Names(sts))

	db.DPrintf(db.TEST, "named %v", sp.Names(sts))

	sts, err = ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "New realm sigmaclnt contents: %v", sp.Names(sts))

	db.DPrintf(db.TEST, "named root %v", sp.Names(sts))

	// Wait for a bit for the crash semaphore to be crated
	time.Sleep(CRASH_SEM_DELAY)

	// Crash named
	db.DPrintf(db.TEST, "Crashing named %v", sp.Names(sts))
	err = crash.SignalFailer(sc.FsLib, fn)
	assert.Nil(t, err, "Err crash: %v", err)
	time.Sleep(DELAY)

	db.DPrintf(db.TEST, "named should be dead & buried")

	_, err = sc.GetDir(sp.NAMED)
	assert.NotNil(t, err)

	db.DPrintf(db.TEST, "named unreachable, as expected")

	// Start a new named
	nd2 := newNamedProc(MCPU, test.REALM1, ts.ProcEnv().UseDialProxy, false)
	db.DPrintf(db.TEST, "Starting a new named: %v", nd2.GetPid())
	if err := startNamed(ts, nd2); !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
		return
	}

	sts, err = sc.GetDir(sp.NAMED + "/")
	assert.Nil(t, err, "Get named dir post-crash")
	db.DPrintf(db.TEST, "named %v", sp.Names(sts))

	if err := stopNamed(ts, nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}
}

// Create a leased file and then reboot
func reboot(t *testing.T, dn string, f func(*test.Tstate, *sigmaclnt.SigmaClnt, string), d time.Duration, quick bool) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nd1 := newNamedProc(MCPU, test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := startNamed(ts, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Made new realm sigmaclnt")

	fn := filepath.Join(dn, "leasedf")

	li, err := sc.LeaseClnt.AskLease(fn, fsetcd.LeaseTTL)
	assert.Nil(t, err, "Error AskLease: %v", err)

	_, err = sc.PutLeasedFile(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err, "Err PutLeasedFile: %v", err)

	sts, err := sc.GetDir(dn)
	assert.Nil(ts.T, err)

	assert.Equal(ts.T, 1, len(sts))

	stopNamed(ts, nd1)
	ts.Shutdown()

	if !quick {
		// reboot after a while
		time.Sleep(d)
	}

	ts, err1 = test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	nd2 := newNamedProc(MCPU, test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := startNamed(ts, nd2); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	if quick {
		// if we rebooted quickly, wait now for a while
		time.Sleep(d)
	}

	pe2 := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc2, err := sigmaclnt.NewSigmaClnt(pe2)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}

	f(ts, sc2, fn)

	ts.Remove(fn)

	stopNamed(ts, nd2)
}

// In these tests named will receive notification from etcd that
// leased file has expired.
func TestLeaseQuickReboot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nd1 := newNamedProc(MCPU, test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := startNamed(ts, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Made new realm sigmaclnt")

	dn := filepath.Join(sp.NAMED, "dir")
	sc.RmDir(dn)
	err = sc.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	// Verify the dir was made correctly
	sts, err := sc.GetDir(dn)
	assert.Nil(t, err, "Err GetDir: %v", err)
	assert.Equal(t, 0, len(sts))
	err = stopNamed(ts, nd1)
	// Shut down regardless of whether or not stopping named was successful
	ts.Shutdown()
	if !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}

	delay := 2 * fsetcd.LeaseTTL * time.Second

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		sts, err := sc.GetDir(dn)
		assert.Nil(t, err, "Err GetDir: %v", err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v", err)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		fd, err := sc.Create(fn, 0777, sp.OREAD)
		assert.Nil(ts.T, err, "Err Create: %v", err)
		db.DPrintf(db.TEST, "Create after expire err %v", err)
		sc.CloseFd(fd)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		fd, err := sc.Create(fn, 0777, sp.OREAD)
		assert.NotNil(ts.T, err, "Unexpected nil err create")
		db.DPrintf(db.TEST, "Create before expire err %v", err)
		sc.CloseFd(fd)
	}, (fsetcd.LeaseTTL-3)*time.Second, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Remove(fn)
		assert.NotNil(ts.T, err, "Unexpected nil err remove")
		db.DPrintf(db.TEST, "Remove after expire err %v", err)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err, "Unexpected nil err rename")
		db.DPrintf(db.TEST, "Rename after expire err %v", err)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		_, err = sc.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err, "Unexpected nil err open")
		db.DPrintf(db.TEST, "Open after expire err %v", err)
		sc.Remove(fn)
	}, delay, true)
}

// In these tests named will not receive notification from etcd when
// leased file expires, but discovers it when reading from etcd and
// call updateDir.
func TestLeaseDelayReboot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(sp.NAMED, "ddd")
	ts.RmDir(dn)
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	ts.Shutdown()

	delay := 2 * fsetcd.LeaseTTL * time.Second

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		sts, err := sc.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v", err)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		fd, err := sc.Create(fn, 0777, sp.OREAD)
		assert.Nil(ts.T, err)
		db.DPrintf(db.TEST, "Create after expire err %v", err)
		sc.CloseFd(fd)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Remove(fn)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Remove after expire err %v", err)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Rename after expire err %v", err)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		_, err = sc.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Open after expire err %v", err)
		sc.Remove(fn)
	}, delay, false)
}

// Test if read fails after a named lost leadership
func TestPartitionNamed(t *testing.T) {
	e := crash.NewEventStartDelay(crash.NAMED_PARTITION, 2000, 1000, 7000, float64(1.0))
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pe := ts.ProcEnv()
	npc := dialproxyclnt.NewDialProxyClnt(pe)
	ep, err := fsetcd.GetRootNamed(npc.Dial, pe.GetEtcdEndpoints(), pe.GetRealm())
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "ep named1 %v", ep)

	dn := filepath.Join(sp.NAMED, "ddd")
	ts.RmDir(dn)
	err = ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	fn := filepath.Join(dn, "fff")

	_, err = ts.PutFile(fn, 0777, sp.OWRITE, []byte("hello"))
	assert.Nil(t, err, "Err PutFile: %v", err)

	b, err := ts.GetFile(fn)
	assert.Nil(t, err)
	assert.Equal(t, len(b), 5)

	rdr, err := ts.OpenReader(fn)
	assert.Nil(t, err)

	// start second named but without SIGMAFAIL
	err = ts.BootEnv(sp.NAMEDREL, []string{"SIGMAFAIL="})
	assert.Nil(t, err)

	// give the first named chance to partition
	time.Sleep(time.Duration(e.Start+e.MaxInterval+e.Delay) * time.Millisecond)

	// wait until session times out
	time.Sleep(sp.EtcdSessionTTL * time.Second)

	pe.ClearNamedEndpoint()
	npc = dialproxyclnt.NewDialProxyClnt(pe)
	ep, err = fsetcd.GetRootNamed(npc.Dial, pe.GetEtcdEndpoints(), pe.GetRealm())
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "ep named2 %v", ep)

	// put to second named
	pe = proc.NewAddedProcEnv(ts.ProcEnv())
	pe.SetNamedEndpoint(ep)
	fsl2, err := sigmaclnt.NewFsLib(pe, ts.GetDialProxyClnt())

	_, err = fsl2.PutFile(filepath.Join(dn, "ggg"), 0777, sp.OWRITE, []byte("bye"))
	assert.Nil(t, err, "Err PutFile: %v", err)

	sts, err := fsl2.GetDir(dn)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(sts))

	// read from old server
	sts, err = ts.GetDir(dn)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(sts))

	// read from old server
	b = make([]byte, 1)
	n, err := rdr.Read(b)
	assert.NotNil(t, err)
	assert.NotEqual(t, 1, n)
	db.DPrintf(db.TEST, "read err %v", err)

	// wait until first named has exited
	time.Sleep(time.Duration(e.Delay) * time.Millisecond)

	ts.Shutdown()
}
