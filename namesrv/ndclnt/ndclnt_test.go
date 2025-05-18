package ndclnt_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv"
	"sigmaos/namesrv/fsetcd"
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
)

func TestCompile(t *testing.T) {
}

func TestBootNamed(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts, err := ts.GetDir(sp.NAMED)
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

func TestKillNamedAlone(t *testing.T) {
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

	nd1 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
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

	sts, err := sc.GetDir(sp.NAMED)
	assert.Nil(t, err)
	assert.True(t, sp.Present(sts, []string{"fff"}))

	sts, err = ts.GetDir(sp.NAMED)
	assert.Nil(t, err)
	// db.DPrintf(db.TEST, "Root realm sigmaclnt contents: %v", sp.Names(sts))
	assert.True(t, sp.Present(sts, []string{"chunkd"}))

	// Wait for a bit for the crash semaphore to be created
	time.Sleep(CRASH_SEM_DELAY)

	// Tell named to crash
	db.DPrintf(db.TEST, "Crashing named")
	err = crash.SignalFailer(sc.FsLib, crashpn)
	assert.Nil(t, err, "Err crash: %v", err)

	// Allow the crashing named time to crash
	time.Sleep(T * time.Millisecond)

	db.DPrintf(db.TEST, "named should be dead & buried")

	_, err = sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.NotNil(t, err)

	db.DPrintf(db.TEST, "named unreachable, as expected")

	// Start a new named
	nd2 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, false)
	db.DPrintf(db.TEST, "Starting a new named: %v", nd2.GetPid())
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd2); !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
		return
	}

	fn1 := filepath.Join(sp.NAMED, "ggg")
	_, err = sc.PutFile(fn1, 0777, sp.OREAD, nil)
	assert.Nil(t, err)

	sts, err = sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.Nil(t, err, "Get named dir post-crash")
	assert.True(t, sp.Present(sts, []string{"fff"}))
	assert.True(t, sp.Present(sts, []string{"ggg"}))

	err = sc.Remove(fn)
	assert.Equal(t, nil, err)
	err = sc.Remove(fn1)
	assert.Equal(t, nil, err)

	if err := ndclnt.StopNamed(ts.SigmaClnt, nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}
}

func TestKillNamedClient(t *testing.T) {
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

	nd1 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}

	ch := make(chan bool)
	go func(ch chan bool) {
		done := false
		for !done {
			select {
			case <-ch:
				db.DPrintf(db.TEST, "done")
				done = true
			default:
				time.Sleep(1 * time.Second)
				db.DPrintf(db.TEST, "Create")
				fn := filepath.Join(sp.NAMED, "fff")
				_, err = sc.PutFile(fn, 0777, sp.OREAD, nil)
				assert.Nil(t, err)
				sts, err := sc.GetDir(sp.NAMED)
				assert.Nil(t, err)
				assert.True(t, sp.Present(sts, []string{fn}))
				err = sc.Remove(fn)
				assert.Equal(t, nil, err)
			}
		}
		ch <- true
	}(ch)

	time.Sleep(3 * time.Second)

	// Start a new named
	nd2 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, false)
	db.DPrintf(db.TEST, "Starting a new named: %v", nd2.GetPid())
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd2); !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
		return
	}
	// Tell named to old crash
	db.DPrintf(db.TEST, "Crashing named")
	err = crash.SignalFailer(sc.FsLib, crashpn)
	assert.Nil(t, err, "Err crash: %v", err)

	// Allow the crashing named time to crash
	time.Sleep(T * time.Millisecond)

	db.DPrintf(db.TEST, "named should be dead & buried")

	ch <- true
	<-ch

	db.DPrintf(db.TEST, "client finished")

	if err := ndclnt.StopNamed(ts.SigmaClnt, nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}
}

// Create a leased file and then reboot
func reboot(t *testing.T, dn string, f func(*test.Tstate, *sigmaclnt.SigmaClnt, string), quick bool) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nd1 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Made new realm sigmaclnt")

	fn := filepath.Join(dn, "leasedf")

	ttl := sp.Tttl(7 * fsetcd.LeaseTTL)
	d := time.Duration(ttl+1) * time.Second
	li, err := sc.LeaseClnt.AskLease(fn, ttl)
	assert.Nil(t, err, "Error AskLease: %v", err)

	_, err = sc.PutLeasedFile(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err, "Err PutLeasedFile: %v", err)

	sts, err := sc.GetDir(dn)
	assert.Nil(ts.T, err)

	assert.Equal(ts.T, 1, len(sts))

	ndclnt.StopNamed(ts.SigmaClnt, nd1)
	ts.Shutdown()

	if !quick {
		// Wait for the lease to expire before rebooting
		time.Sleep(d)
	}

	ts, err1 = test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	nd2 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd2); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe2 := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc2, err := sigmaclnt.NewSigmaClnt(pe2)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}

	if quick {
		// Make sure the leased file still exists
		sts, err := sc2.GetDir(dn)
		assert.Nil(t, err, "Err GetDir: %v", err)
		assert.Equal(t, 1, len(sts), "Leased file expired during reboot (before expected)")
		// if we rebooted quickly, wait for the lease to expire now
		time.Sleep(d)
		pe3 := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
		sc3, err := sigmaclnt.NewSigmaClnt(pe3)
		if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
			return
		}
		// Make sure the leased file no longer exists
		_, err = sc3.GetFile(fn)
		assert.NotNil(t, err, "GetFile err should not be nil: %v", err)
	}

	f(ts, sc2, fn)

	ts.Remove(fn)

	ndclnt.StopNamed(ts.SigmaClnt, nd2)
}

// In these tests named will receive notification from etcd that
// leased file has expired.
func TestLeaseQuickReboot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nd1 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
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
	err = ndclnt.StopNamed(ts.SigmaClnt, nd1)
	// Shut down regardless of whether or not stopping named was successful
	ts.Shutdown()
	if !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		sts, err := sc.GetDir(dn)
		assert.Nil(t, err, "Err GetDir: %v", err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v", err)
	}, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		fd, err := sc.Create(fn, 0777, sp.OREAD)
		assert.Nil(ts.T, err, "Err Create: %v", err)
		db.DPrintf(db.TEST, "Create after expire err %v", err)
		sc.CloseFd(fd)
		err = sc.Remove(fn)
		assert.Nil(ts.T, err, "Err remove: %v", err)
	}, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Remove(fn)
		assert.NotNil(ts.T, err, "Unexpected nil err remove")
		db.DPrintf(db.TEST, "Remove after expire err %v", err)
	}, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err, "Unexpected nil err rename")
		db.DPrintf(db.TEST, "Rename after expire err %v", err)
	}, true)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		_, err = sc.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err, "Unexpected nil err open")
		db.DPrintf(db.TEST, "Open after expire err %v", err)
		sc.Remove(fn)
	}, true)
}

// In these tests named will not receive notification from etcd when
// leased file expires, but discovers it when reading from etcd and
// call updateDir.
func TestLeaseDelayReboot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nd1 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Made new realm sigmaclnt")

	dn := filepath.Join(sp.NAMED, "ddd")
	sc.RmDir(dn)
	err = sc.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	// Verify the dir was made correctly
	sts, err := sc.GetDir(dn)
	assert.Nil(t, err, "Err GetDir: %v", err)
	assert.Equal(t, 0, len(sts))
	err = ndclnt.StopNamed(ts.SigmaClnt, nd1)
	// Shut down regardless of whether or not stopping named was successful
	ts.Shutdown()
	if !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		sts, err := sc.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v", err)
	}, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		fd, err := sc.Create(fn, 0777, sp.OREAD)
		assert.Nil(ts.T, err)
		db.DPrintf(db.TEST, "Create after expire err %v", err)
		sc.CloseFd(fd)
		err = sc.Remove(fn)
		assert.Nil(ts.T, err, "Err remove: %v", err)
	}, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Remove(fn)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Remove after expire err %v", err)
	}, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		err := sc.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Rename after expire err %v", err)
	}, false)

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		_, err = sc.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Open after expire err %v", err)
		sc.Remove(fn)
	}, false)
}

// In these tests named will not receive notification from etcd when
// leased file expires, but discovers it when reading from etcd and
// call updateDir.
func TestLeaseGetDirReboot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nd1 := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := ndclnt.StartNamed(ts.SigmaClnt, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if !assert.Nil(ts.T, err, "Err new sigmaclnt realm: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Made new realm sigmaclnt")

	dn := filepath.Join(sp.NAMED, "thedir")
	sc.RmDir(dn)
	err = sc.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	// Verify the dir was made correctly
	sts, err := sc.GetDir(dn)
	assert.Nil(t, err, "Err GetDir: %v", err)
	assert.Equal(t, 0, len(sts))
	err = ndclnt.StopNamed(ts.SigmaClnt, nd1)
	// Shut down regardless of whether or not stopping named was successful
	ts.Shutdown()
	if !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}

	reboot(t, dn, func(ts *test.Tstate, sc *sigmaclnt.SigmaClnt, fn string) {
		sts, err := sc.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		sts, err = sc.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire ok")
	}, true)
}

// Test if read fails after a named lost leadership
// XXX commented out until etcd fix
func xTestPartitionNamed(t *testing.T) {
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
