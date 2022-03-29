package sessionclnt_test

import (
	"testing"

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
)

func TestServerCrash(t *testing.T) {
	ts := test.MakeTstateAll(t)

	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 1, CRASH, 0)

	_, err := ts.Stat(DIRGRP0)
	assert.Nil(t, err)

	sem := semclnt.MakeSemClnt(ts.FsLib, DIRGRP0+"sem")
	sem.Init(0)

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

func TestSessClose(t *testing.T) {
	ts := test.MakeTstateAll(t)

	grp := groupmgr.Start(ts.FsLib, ts.ProcClnt, 0, "bin/user/kvd", []string{GRP0}, 0, 0, PARTITION)

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
