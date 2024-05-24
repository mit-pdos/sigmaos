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

	sts, err = ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err, "Err GetDir: %v", err)
	db.DPrintf(db.TEST, "named %v\n", sp.Names(sts))

	ts.Shutdown()
}

func reboot(t *testing.T, dn string, f func(*test.Tstate, string)) {
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

	// Wait for ephemeral file to expire
	time.Sleep(fsetcd.LeaseTTL*time.Second + 1)

	f(ts, fn)

	ts.Shutdown()
}

func TestEphemeralReboot(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(sp.NAMED, "dir")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	ts.Shutdown()

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Remove(fn)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Remove after expire err %v\n", err)
	})

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		err := ts.Rename(fn, fn+"x")
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Rename after expire err %v\n", err)
	})

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		_, err = ts.Open(fn, sp.OREAD)
		assert.NotNil(ts.T, err)
		db.DPrintf(db.TEST, "Open after expire err %v\n", err)
		ts.Remove(fn)
	})

	reboot(t, dn, func(ts *test.Tstate, fn string) {
		sts, err := ts.GetDir(dn)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(sts))
		db.DPrintf(db.TEST, "GetDir after expire err %v\n", err)
	})
}
