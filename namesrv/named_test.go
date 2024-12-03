package namesrv_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/util/crash"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
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
	db.DPrintf(db.TEST, "named %v\n", sp.Names(sts))

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
	db.DPrintf(db.TEST, "pstats %v\n", st)

	ts.Shutdown()
}

func TestKillNamed(t *testing.T) {
	const T = 1000
	fn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPath(crash.NAMED_CRASH, T, 1.0, fn)
	err := crash.SetSigmaFail([]crash.Tevent{e})
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts, err := ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "named %v\n", sp.Names(sts))

	err = ts.BootEnv(sp.NAMEDREL, []string{"SIGMAFAIL="})
	assert.Nil(t, err)

	sts, err = ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "named %v\n", sp.Names(sts))

	db.DPrintf(db.TEST, "Crash named..\n")

	sem := semclnt.NewSemClnt(ts.FsLib, fn)
	err := sem.Up()
	assert.Nil(t, err)
	time.Sleep(T * time.Millisecond)

	db.DPrintf(db.TEST, "GetDir..\n")

	n := 0
	for i := 0; i < sp.Conf.Path.MAX_RESOLVE_RETRY; i++ {
		n = i
		sts, err = ts.GetDir(sp.NAMED + "/")
		if err == nil {
			break
		}
	}
	assert.Nil(t, err, "Err GetDir: %v", err)
	db.DPrintf(db.TEST, "named tries %d %v\n", n, sp.Names(sts))

	ts.Shutdown()
}

// Create a leased file and then reboot
func reboot(t *testing.T, dn string, f func(*test.Tstate, string), d time.Duration, quick bool) {
	ts, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(dn, "leasedf")

	li, err := ts.LeaseClnt.AskLease(fn, fsetcd.LeaseTTL)
	assert.Nil(t, err, "Error AskLease: %v", err)

	_, err = ts.PutLeasedFile(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err, "Err PutLeasedFile: %v", err)

	sts, err := ts.GetDir(dn)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(sts))

	ts.Shutdown()

	if !quick {
		// reboot after a while
		time.Sleep(d)
	}

	ts, err1 = test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	if quick {
		// if we rebooted quickly, wait now for a while
		time.Sleep(d)
	}

	f(ts, fn)

	ts.Remove(fn)

	ts.Shutdown()
}

// In these tests named will receive notification from etcd that
// leased file has expired.
func TestLeaseQuickReboot(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(sp.NAMED, "dir")
	ts.RmDir(dn)
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	ts.Shutdown()

	delay := 2 * fsetcd.LeaseTTL * time.Second

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		sts, err := ts.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v\n", err)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		fd, err := ts.Create(fn, 0777, sp.OREAD)
		assert.Nil(ts.T, err)
		db.DPrintf(db.TEST, "Create after expire err %v\n", err)
		ts.CloseFd(fd)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		fd, err := ts.Create(fn, 0777, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Create before expire err %v\n", err)
		ts.CloseFd(fd)
	}, (fsetcd.LeaseTTL-3)*time.Second, true)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Remove(fn)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Remove after expire err %v\n", err)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Rename after expire err %v\n", err)
	}, delay, true)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		_, err = ts.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Open after expire err %v\n", err)
		ts.Remove(fn)
	}, delay, true)
}

// In these tests named will not receive notification from etcd when
// leased file expires, but discovers it when reading from etcd and
// call updateDir.
func TestLeaseDelayReboot(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(sp.NAMED, "ddd")
	ts.RmDir(dn)
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	ts.Shutdown()

	delay := 2 * fsetcd.LeaseTTL * time.Second

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		sts, err := ts.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v\n", err)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		fd, err := ts.Create(fn, 0777, sp.OREAD)
		assert.Nil(ts.T, err)
		db.DPrintf(db.TEST, "Create after expire err %v\n", err)
		ts.CloseFd(fd)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Remove(fn)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Remove after expire err %v\n", err)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Rename after expire err %v\n", err)
	}, delay, false)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		_, err = ts.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Open after expire err %v\n", err)
		ts.Remove(fn)
	}, delay, false)
}

// Test if read fails after a named lost leadership
func TestPartitionNamed(t *testing.T) {
	e := crash.NewEventStartDelay(crash.NAMED_PARTITION, 2000, 1000, 1.0, 7000)
	err := crash.SetSigmaFail([]crash.Tevent{e})
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
	time.Sleep(time.Duration(e.Start+e.MaxInterval) * time.Millisecond)

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
