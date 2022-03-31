package sessclnt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	"ulambda/semclnt"
	"ulambda/test"
)

const (
	GRP0      = "GRP0"
	DIRGRP0   = group.GRPDIR + GRP0 + "/"
	CRASH     = 1000
	PARTITION = 1000
	NETFAIL   = 200
)

func TestServerCrash(t *testing.T) {
	ts := test.MakeTstateAll(t)

	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 1, CRASH, 0, 0)

	sem := semclnt.MakeSemClnt(ts.FsLib, DIRGRP0+"sem")
	err := sem.Init(0)
	assert.Nil(t, err)

	ch := make(chan error)
	go func() {
		fsl := fslib.MakeFsLibAddr("fslibtest-1", fslib.Named())
		sem := semclnt.MakeSemClnt(fsl, DIRGRP0+"sem")
		err := sem.Down()
		ch <- err
	}()

	err = <-ch
	assert.NotNil(ts.T, err, "down")

	grp.Stop()

	ts.Shutdown()
}

func TestReconnectSimple(t *testing.T) {
	const N = 1000
	ts := test.MakeTstateAll(t)
	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 0, 0, 0, NETFAIL)
	ch := make(chan error)
	go func() {
		fsl := fslib.MakeFsLibAddr("fslibtest-1", fslib.Named())
		for i := 0; i < N; i++ {
			_, err := fsl.Stat(DIRGRP0)
			if err != nil {
				ch <- err
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		ch <- nil
	}()

	err := <-ch
	assert.Nil(ts.T, err, "fsl1")

	grp.Stop()
	ts.Shutdown()
}

func TestAutoSessClose(t *testing.T) {
	ts := test.MakeTstateAll(t)
	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 0, 0, PARTITION, 0)

	sem := semclnt.MakeSemClnt(ts.FsLib, DIRGRP0+"sem")
	sem.Init(0)

	ch := make(chan error)
	go func() {
		fsl := fslib.MakeFsLibAddr("fslibtest-1", fslib.Named())
		sem := semclnt.MakeSemClnt(fsl, DIRGRP0+"sem")
		err := sem.Down()
		ch <- err
	}()

	err := <-ch
	assert.NotNil(ts.T, err, "down")

	grp.Stop()

	ts.Shutdown()
}
