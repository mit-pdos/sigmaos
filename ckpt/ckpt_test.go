package ckpt_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	NPAGES  = "1000"
	PROGRAM = "ckpt-proc"
)

func TestSpawnCkpt(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	os.Remove("/tmp/sigmaos-perf/log.txt")

	run := 10

	pid := sp.GenPid(PROGRAM)
	pn := sp.UX + "~any/" + pid.String() + "/"

	ckptProc := proc.NewProcPid(pid, PROGRAM, []string{strconv.Itoa(run), NPAGES, pn})
	err = ts.Spawn(ckptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(ckptProc.GetPid())
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until proc %v has checkpointed itself", ckptProc.GetPid())

	status, err := ts.WaitExit(ckptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusErr())

	time.Sleep(100 * time.Millisecond)

	pid = sp.GenPid("ckpt-proc-copy")

	db.DPrintf(db.TEST, "Spawn from checkpoint %v", pid)

	restProc := proc.NewProcFromCheckpoint(pid, "ckpt-proc-copy", pn)
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until start")

	err = ts.WaitStart(restProc.GetPid())
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait %v until exit", restProc.GetPid())

	status, err = ts.WaitExit(restProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}
