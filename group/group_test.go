package group

import (
	//"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/coordmgr"
	"ulambda/kernel"
	//"ulambda/proc"
	//"ulambda/procclnt"
)

type Tstate struct {
	*kernel.System
	t  *testing.T
	gm *coordmgr.CoordMgr
}

func makeTstate(t *testing.T, crash int) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemAll("mfsgrp_test", "..")
	ts.gm = coordmgr.StartCoords(ts.System.FsLib, ts.System.ProcClnt, 1, "bin/user/kvd", []string{GRP + "0"}, 0)
	return ts
}

func TestOne(t *testing.T) {
	ts := makeTstate(t, 0)
	err := ts.gm.StopCoords()
	assert.Nil(ts.t, err, "Stop")
	ts.Shutdown()
}
