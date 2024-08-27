package ckpt_test

import (
	"os"
	"strconv"
	"strings"
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

func TestNoCkpt(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	pid := sp.GenPid(PROGRAM)
	pn := sp.UX + "~any/" + pid.String() + "/"

	chkptProc := proc.NewProcPid(pid, PROGRAM, []string{"no", "10", NPAGES, pn})
	err = ts.Spawn(chkptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(chkptProc.GetPid())
	assert.Nil(t, err)

	status, err := ts.WaitExit(chkptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

func TestExtCkpt(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	os.Remove("/tmp/sigmaos-perf/log.txt")

	run := 10
	pid := sp.GenPid(PROGRAM)
	pn := sp.UX + "~any/" + pid.String() + "/"

	chkptProc := proc.NewProcPid(pid, PROGRAM, []string{"ext", strconv.Itoa(run), NPAGES, pn})
	err = ts.Spawn(chkptProc)
	assert.Nil(t, err)

	// let ckpt-proc run for a little while
	time.Sleep(time.Duration(run/2) * time.Second)

	db.DPrintf(db.TEST, "checkpointing %q", pn)
	err = ts.Checkpoint(chkptProc.GetPid(), pn)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "checkpoint err %v", err)

	restProc := proc.NewProcFromCheckpoint(pid, "ckpt-proc-copy", pn)
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	n := time.Duration(time.Duration(run/2+3) * time.Second)
	db.DPrintf(db.TEST, "sleep for a while %v", n)
	time.Sleep(n)

	dots := make([]byte, run)
	for i := 0; i < run; i++ {
		dots[i] = '.'
	}
	b, err := os.ReadFile("/tmp/sigmaos-perf/log.txt")
	db.DPrintf(db.TEST, "b %v\n", string(b))
	assert.True(t, strings.Contains(string(b), string(dots)))
	assert.True(t, strings.Contains(string(b), "exit"))

	ts.Shutdown()
}

func TestSelfCkpt(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	os.Remove("/tmp/sigmaos-perf/log.txt")

	run := 10

	pid := sp.GenPid(PROGRAM)
	pn := sp.UX + "~any/" + pid.String() + "/"

	ckptProc := proc.NewProcPid(pid, PROGRAM, []string{"self", strconv.Itoa(run), NPAGES, pn})
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
