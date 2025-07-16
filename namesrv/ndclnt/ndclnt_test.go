package ndclnt_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	// dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/namesrv/ndclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/coordination/semaphore"
	"sigmaos/util/crash"
	"sigmaos/util/retry"
)

const (
	MCPU            proc.Tmcpu = 1000
	CRASH_SEM_DELAY            = 100 * time.Millisecond
	CRASHFILE                  = "###crashfile##!"
)

func TestCompile(t *testing.T) {
}

func TestBootKnamed(t *testing.T) {
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
	assert.True(t, len(st.Counters) > 0)
	db.DPrintf(db.TEST, "pstats %v", st)

	ts.Shutdown()
}

func makeNamed1(ts *test.Tstate) (*ndclnt.NdClnt, *proc.Proc, error) {
	nd := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	ndc, err := ndclnt.NewNdClnt(ts.SigmaClnt, test.REALM1)
	if err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.TEST, "Starting named1: %v", nd.GetPid())
	return ndc, nd, ndc.ClearAndStartNamed(nd)
}

func TestBootNamed(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	_, _, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}
	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	assert.Nil(t, err)
	sts, err := sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.Nil(t, err)
	assert.True(t, sp.Present(sts, []string{"rpc"}))
}

// Test many clients mounting two servers concurrently, mimicing
// [procclnt/initproc].
func TestManyClients(t *testing.T) {
	const N = 400

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	_, _, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}
	c := make(chan bool)
	sts, err := ts.GetDir(sp.MSCHED)
	assert.Nil(t, err)
	kernelId := sts[0].Name
	pn := filepath.Join(sp.MSCHED, kernelId)
	rpcep, err := ts.ReadEndpoint(pn)
	for i := 0; i < N; i++ {
		go func() {
			pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
			sc, err := sigmaclnt.NewSigmaClnt(pe)
			mschedC := make(chan error)
			namedC := make(chan error)
			go func() {
				sts, err := sc.GetDir(path.MarkResolve(sp.NAMED))
				if err != nil {
					namedC <- err
					return
				}
				assert.True(t, sp.Present(sts, []string{"rpc"}))
				namedC <- nil
			}()
			go func() {
				err := sc.MountTree(rpcep, "", pn)
				if err != nil {
					mschedC <- err
					return
				}
				sts, err := sc.GetDir(path.MarkResolve(pn))
				assert.True(t, len(sts) > 0)
				mschedC <- err
			}()
			err = <-namedC
			assert.Nil(t, err)
			err = <-mschedC
			assert.Nil(t, err)
			sc.Close()
			c <- true
		}()
	}
	for i := 0; i < N; i++ {
		<-c
	}
}

func makeNamed2(ts *test.Tstate, ndc *ndclnt.NdClnt, wait, canFail bool) (*proc.Proc, error) {
	nd := ndclnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, canFail)
	db.DPrintf(db.TEST, "Starting named2: %v", nd.GetPid())
	if wait {
		return nd, ndc.ClearAndStartNamed(nd)
	} else {
		return nd, ndc.StartNamed(nd)
	}
}

func TestCrashNamedAlone(t *testing.T) {
	const T = 200
	crashpn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPath(crash.NAMED_CRASH, 0, float64(1.0), crashpn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	ndc, _, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}

	sc := ndc.SigmaClntRealm()
	fn := filepath.Join(sp.NAMED, "fff")
	_, err = sc.PutFile(fn, 0777, sp.OREAD, nil)
	assert.Nil(t, err)

	sts, err := sc.GetDir(sp.NAMED)
	assert.Nil(t, err)
	assert.True(t, sp.Present(sts, []string{"fff"}))

	sts, err = ts.GetDir(sp.NAMED)
	assert.Nil(t, err)
	//db.DPrintf(db.TEST, "Root realm sigmaclnt contents: %v", sp.Names(sts))
	assert.True(t, sp.Present(sts, []string{"chunkd"}))

	// Wait for a bit for the crash semaphore to be created
	time.Sleep(CRASH_SEM_DELAY)

	// Tell named to crash
	err = crash.SignalFailer(sc.FsLib, crashpn)
	assert.Nil(t, err, "Err crash: %v", err)

	// Allow the crashing named time to crash
	time.Sleep(T * time.Millisecond)

	_, err = sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.NotNil(t, err)

	// Start a new named
	nd2, err := makeNamed2(ts, ndc, true, false)
	if !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
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

	if err := ndc.StopNamed(nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}
}

func namedClient(t *testing.T, sc *sigmaclnt.SigmaClnt, ch chan bool) {
	const MAXRETRY = 30
	done := false
	for !done {
		select {
		case <-ch:
			done = true
		default:
			time.Sleep(10 * time.Millisecond)
			d := []byte("hello")
			fn := filepath.Join(sp.NAMED, "fff")
			_, err := sc.PutFile(fn, 0777, sp.OWRITE|sp.OEXCL, d)
			if err == nil {
				err, ok := retry.RetryAtLeastOnce(func() error {
					sts, err := sc.GetDir(sp.NAMED)
					if err == nil {
						assert.True(t, sp.Present(sts, []string{fn}))
					}
					return err
				})
				assert.Nil(t, err)
				assert.True(t, ok)
				err, ok = retry.RetryAtLeastOnce(func() error {
					dg, err := sc.GetFile(fn)
					if err == nil {
						assert.Equal(t, d, dg)
					}
					return err
				})
				assert.Nil(t, err)
				assert.True(t, ok)
				_, ok = retry.RetryAtLeastOnce(func() error {
					err := sc.Remove(fn)
					return err
				})
				assert.True(t, ok)
			}
		}
	}
	ch <- true
}

func namedClientBlocking(t *testing.T, sc *sigmaclnt.SigmaClnt, ch chan bool) {
	pn := filepath.Join(sp.NAMED, "crash.sem")
	done := false
	for !done {
		select {
		case <-ch:
			done = true
		default:
			time.Sleep(10 * time.Millisecond)
			sem := semaphore.NewSemaphore(sc.FsLib, pn)
			if err := sem.Init(0); err != nil {
				db.DPrintf(db.TEST, "init err %v", err)
			}
			if err := sem.Down(); err != nil {
				db.DPrintf(db.TEST, "down err %v", err)
			}
		}
	}
	ch <- true
}

func TestCrashNamedClient(t *testing.T) {
	const (
		T = 200
		N = 5
	)

	crashpn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPath(crash.NAMED_CRASH, T, float64(1.0), crashpn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	ndc, _, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}
	sc := ndc.SigmaClntRealm()

	for i := 0; i < N; i++ {
		ch := make(chan bool)
		go namedClient(t, sc, ch)

		// Let namedClient do a few iterations
		time.Sleep(1 * time.Second)

		// Start a new named
		nd2, err := makeNamed2(ts, ndc, false, true)
		if !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
			return
		}

		// Wait for a bit for semaphores to be created
		time.Sleep(CRASH_SEM_DELAY)

		// Tell named to old crash
		err = crash.SignalFailer(sc.FsLib, crashpn)
		assert.Nil(t, err, "Err crash: %v", err)

		// Allow the crashing named time to crash
		time.Sleep(T * time.Millisecond)

		ch <- true
		<-ch

		if i == N-1 {
			db.DPrintf(db.TEST, "client finished")
			if err := ndc.StopNamed(nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
				return
			}
		}
	}
}

func testReconnectClient(t *testing.T, f func(t *testing.T, sc *sigmaclnt.SigmaClnt)) {
	const (
		N       = 5
		NETFAIL = 200
	)

	e := crash.NewEvent(crash.NAMED_NETFAIL, NETFAIL, 0.33)
	em := crash.NewTeventMapOne(e)
	e1 := crash.NewEvent(crash.NAMED_NETDISCONNECT, NETFAIL, 0.33)
	em.Insert(e1)
	err := crash.SetSigmaFail(em)
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	ndc, nd1, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}

	f(t, ndc.SigmaClntRealm())

	err = ndc.StopNamed(nd1)
	assert.Nil(t, err)
}

func TestReconnectClientNonBlocking(t *testing.T) {
	testReconnectClient(t, func(t *testing.T, sc *sigmaclnt.SigmaClnt) {
		ch := make(chan bool)
		go namedClient(t, sc, ch)
		// Let namedClient experience network failures
		time.Sleep(10 * time.Second)
		ch <- true
		<-ch
	})
}

func TestReconnectClientBlocking(t *testing.T) {
	testReconnectClient(t, func(t *testing.T, sc *sigmaclnt.SigmaClnt) {
		ch := make(chan bool)
		go namedClientBlocking(t, sc, ch)
		// Let namedClient experience network failures
		time.Sleep(10 * time.Second)
		ch <- true
		<-ch
	})
}

func TestAtMostOnce(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	e := crash.NewEventPath(crash.NAMED_CRASHFILE, 0, 0.0, CRASHFILE)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ndc, _, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}

	sc := ndc.SigmaClntRealm()

	// Start a hot-standby named
	nd2, err := makeNamed2(ts, ndc, false, false)
	if !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
		return
	}

	_, err = sc.GetDir(sp.NAMED)
	assert.Nil(t, err)

	st, err := sc.Stats()
	assert.Nil(t, err)

	d := []byte("hello")
	fn := filepath.Join(sp.NAMED, CRASHFILE)
	_, err = sc.SetFile(fn, d, sp.OAPPEND, sp.NoOffset)
	assert.NotNil(t, err)
	assert.True(t, serr.IsErrorIO(err))

	var d1 []byte
	err, ok := retry.RetryAtLeastOnce(func() error {
		d1, err = sc.GetFile(fn)
		return err
	})
	assert.Nil(t, err)
	assert.True(t, ok)
	assert.Equal(t, d, d1, d1)

	st1, err := sc.Stats()
	assert.Nil(t, err)

	assert.True(t, st1.Path.Counters["NgetNamedOK"] > st.Path.Counters["NgetNamed"])
	assert.True(t, st1.Path.Counters["Nsession"] > st.Path.Counters["Nsession"])

	if err := ndc.StopNamed(nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}
}

func TestCrashSemaphore(t *testing.T) {
	crashpn := sp.NAMED + "crashnd.sem"
	pn := filepath.Join(sp.NAMED, "crash.sem")

	e := crash.NewEventPath(crash.NAMED_CRASH, 0, float64(1.0), crashpn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	ndc, _, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}

	sc := ndc.SigmaClntRealm()
	sem := semaphore.NewSemaphore(sc.FsLib, pn)
	err = sem.Init(0)
	assert.Nil(t, err)

	sts, err := sc.GetDir(path.MarkResolve(sp.NAMED))
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "named %v", sp.Names(sts))

	ch := make(chan error)
	go func() {
		pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), test.REALM1)
		sc, err := sigmaclnt.NewSigmaClnt(pe)
		sem := semaphore.NewSemaphore(sc.FsLib, pn)
		err = sem.Down()
		ch <- err
	}()

	// Wait for a bit for semaphores to be created
	time.Sleep(CRASH_SEM_DELAY)

	// Tell named storing sem to crash
	err = crash.SignalFailer(sc.FsLib, crashpn)
	assert.Nil(t, err, "Err crash: %v", err)

	err = <-ch
	assert.NotNil(ts.T, err, "down")
}

// Create a leased file and then reboot
func reboot(t *testing.T, dn string, f func(*test.Tstate, *sigmaclnt.SigmaClnt, string), quick bool) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ndc, nd1, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}

	sc := ndc.SigmaClntRealm()

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

	ndc.StopNamed(nd1)
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

	ndc, nd2, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
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

	ndc.StopNamed(nd2)
}

// In these tests named will receive notification from etcd that
// leased file has expired.
func TestLeaseQuickReboot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ndc, nd1, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}
	sc := ndc.SigmaClntRealm()
	dn := filepath.Join(sp.NAMED, "dir")
	sc.RmDir(dn)
	err = sc.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	// Verify the dir was made correctly
	sts, err := sc.GetDir(dn)
	assert.Nil(t, err, "Err GetDir: %v", err)
	assert.Equal(t, 0, len(sts))
	err = ndc.StopNamed(nd1)
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

	ndc, nd1, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}
	sc := ndc.SigmaClntRealm()
	dn := filepath.Join(sp.NAMED, "ddd")
	sc.RmDir(dn)
	err = sc.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	// Verify the dir was made correctly
	sts, err := sc.GetDir(dn)
	assert.Nil(t, err, "Err GetDir: %v", err)
	assert.Equal(t, 0, len(sts))
	err = ndc.StopNamed(nd1)
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

	ndc, nd1, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}

	sc := ndc.SigmaClntRealm()
	dn := filepath.Join(sp.NAMED, "thedir")
	sc.RmDir(dn)
	err = sc.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	// Verify the dir was made correctly
	sts, err := sc.GetDir(dn)
	assert.Nil(t, err, "Err GetDir: %v", err)
	assert.Equal(t, 0, len(sts))
	err = ndc.StopNamed(nd1)
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

func partitionNamed(t *testing.T, delay int64) {
	const DIR = "ddd"
	crashpn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPathDelay(crash.NAMED_PARTITION, 0, delay, float64(1.0), crashpn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ndc, _, err := makeNamed1(ts)
	if !assert.Nil(ts.T, err, "makeNamed err %v", err) {
		return
	}
	sc := ndc.SigmaClntRealm()

	ep1, err := sc.GetNamedEndpoint()
	assert.Nil(t, err)

	dn := filepath.Join(sp.NAMED, DIR)
	sc.RmDir(dn)
	err = sc.MkDir(dn, 0777)
	assert.Nil(t, err, "dir")
	fn := filepath.Join(dn, "fff")

	_, err = sc.PutFile(fn, 0777, sp.OWRITE, []byte("hello"))
	assert.Nil(t, err, "Err PutFile: %v", err)

	b, err := sc.GetFile(fn)
	assert.Nil(t, err)
	assert.Equal(t, len(b), 5)

	rdr, err := sc.OpenReader(fn)
	assert.Nil(t, err)

	nd2, err := makeNamed2(ts, ndc, false, false)
	assert.Nil(t, err)

	// Tell named storing sem to partition
	err = crash.SignalFailer(sc.FsLib, crashpn)
	assert.Nil(t, err, "Err crash: %v", err)

	// wait until session times out
	time.Sleep(sp.EtcdSessionExpired * time.Second)

	ep2, err := ts.ReadEndpoint(ndc.PathName())
	assert.Nil(t, err)

	assert.False(t, ep1.Equal(ep2))

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	pe.SetNamedEndpoint(ep2)
	fsl2, err := sigmaclnt.NewFsLib(pe, ts.GetDialProxyClnt())

	_, err = fsl2.PutFile(filepath.Join(dn, "ggg"), 0777, sp.OWRITE, []byte("bye"))
	assert.Nil(t, err, "Err PutFile: %v", err)

	sts, err := fsl2.GetDir(dn)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(sts))

	// read from old server
	b = make([]byte, 1)
	n, err := rdr.Read(b)
	if delay > 0 {
		// This shouldn't happen but can with current etcd interface.
		assert.Nil(t, err)
		assert.Equal(t, 1, n)
	} else {
		assert.NotNil(t, err)
		assert.NotEqual(t, 1, n)
		if delay < 0 {
			assert.True(t, serr.IsErrorClosed(err))
		} else {
			assert.True(t, serr.IsErrorUnreachable(err))
		}
	}
	err = ndc.StopNamed(nd2)
	assert.Nil(ts.T, err, "Err stop named: %v", err)

	ts.Shutdown()
}

// Test if read fails after a named resigns leadership
func TestPartitionNamedResignOK(t *testing.T) {
	partitionNamed(t, 0)
}

// Test if read fails after a named session expires but before
// resigning.
func TestPartitionNamedExpire(t *testing.T) {
	const DELAY_BAD = (sp.EtcdSessionExpired + 1) * 1000
	partitionNamed(t, -DELAY_BAD)
}

// To mimic possible, incorrect scenario in Expired() (without having
// to modify etcd).
func TestPartitionNamedResignBad(t *testing.T) {
	const DELAY_BAD = (sp.EtcdSessionExpired + 1) * 1000
	partitionNamed(t, DELAY_BAD)
}
