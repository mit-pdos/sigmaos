package group

import (
	//"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/groupmgr"
	"ulambda/kernel"
)

type Tstate struct {
	*kernel.System
	t        *testing.T
	gm       *groupmgr.GroupMgr
	replicas []*kernel.System
}

func (ts *Tstate) Shutdown() {
	ts.System.Shutdown()
	for _, r := range ts.replicas {
		r.Shutdown()
	}
}

func makeTstate(t *testing.T, crash int) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemAll("mfsgrp_test", "..", 0)
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		ts.replicas = append(ts.replicas, kernel.MakeSystemNamed("fslibtest", "..", i+1))
	}
	ts.gm = groupmgr.Start(ts.System.FsLib, ts.System.ProcClnt, 1, "bin/user/kvd", []string{GRP + "0"}, 0)
	return ts
}

func TestOne(t *testing.T) {
	ts := makeTstate(t, 0)
	err := ts.gm.Stop()
	assert.Nil(ts.t, err, "Stop")
	ts.Shutdown()
}
