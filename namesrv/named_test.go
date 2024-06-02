package namesrv_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	"sigmaos/namesrv"
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

	assert.True(t, fslib.Present(sts, namesrv.InitRootDir), "initfs")

	ts.Shutdown()
}

func TestKillNamed(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts, err := ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "named %v\n", sp.Names(sts))

	err = ts.Boot(sp.NAMEDREL)
	assert.Nil(t, err)

	sts, err = ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "named %v\n", sp.Names(sts))

	db.DPrintf(db.TEST, "kill named..\n")

	err = ts.KillOne(sp.NAMEDREL)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "GetDir..\n")

	n := 0
	for i := 0; i < sp.PATHCLNT_MAXRETRY; i++ {
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

func reboot(t *testing.T, dn string, f func(*test.Tstate, string), d time.Duration) {
	ts, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(dn, "ephem")

	li, err := ts.LeaseClnt.AskLease(fn, fsetcd.LeaseTTL)
	assert.Nil(t, err, "Error AskLease: %v", err)

	_, err = ts.PutFileEphemeral(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err, "Err PutEphemeral: %v", err)

	sts, err := ts.GetDir(dn)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(sts))

	// Reboot
	ts.Shutdown()

	ts, err1 = test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	time.Sleep(d)

	f(ts, fn)

	ts.Remove(fn)

	ts.Shutdown()
}

func TestEphemeralReboot(t *testing.T) {
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
		fd, err := ts.Create(fn, 0777, sp.OREAD)
		assert.Nil(ts.T, err)
		db.DPrintf(db.TEST, "Create after expire err %v\n", err)
		ts.CloseFd(fd)
	}, delay)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		fd, err := ts.Create(fn, 0777, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Create before expire err %v\n", err)
		ts.CloseFd(fd)
	}, (fsetcd.LeaseTTL-2)*time.Second)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Remove(fn)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Remove after expire err %v\n", err)
	}, delay)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Rename after expire err %v\n", err)
	}, delay)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		_, err = ts.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Open after expire err %v\n", err)
		ts.Remove(fn)
	}, delay)

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		sts, err := ts.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v\n", err)
	}, delay)
}
