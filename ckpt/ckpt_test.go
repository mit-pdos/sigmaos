package ckpt_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestRunProc(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	chkptProc := proc.NewProc("ckpt-proc", []string{"10"})
	err = ts.Spawn(chkptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(chkptProc.GetPid())
	assert.Nil(t, err)

	status, err := ts.WaitExit(chkptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}

func TestCkptProc(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	os.Remove("/tmp/sigmaos-perf/log.txt")

	chkptProc := proc.NewProc("ckpt-proc", []string{"20"})
	err = ts.Spawn(chkptProc)
	assert.Nil(t, err)
	//err = ts.WaitStart(chkptProc.GetPid())
	//assert.Nil(t, err)

	// let ckpt-proc run for a little while
	time.Sleep(5 * time.Second)

	// pn := sp.S3 + "~any/fkaashoek/" + chkptProc.GetPid().String() + "/"
	pn := sp.UX + "~any/" + chkptProc.GetPid().String() + "/"

	db.DPrintf(db.TEST, "checkpointing %q", pn)
	err = ts.Checkpoint(chkptProc.GetPid(), pn)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "checkpoint err %v", err)

	// spawn and run checkpointed proc
	restProc := proc.NewRestoreProc(chkptProc, pn)
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	n := time.Duration(25)
	db.DPrintf(db.TEST, "sleep for a while %ds", n)
	time.Sleep(n * time.Second)

	//status, err := ts.WaitExit(restProc.GetPid())
	//assert.Nil(t, err)
	//assert.True(t, status.IsStatusOK())

	b, err := os.ReadFile("/tmp/sigmaos-perf/log.txt")
	db.DPrintf(db.TEST, "b %v\n", string(b))
	assert.True(t, strings.Contains(string(b), "........."))
	assert.True(t, strings.Contains(string(b), "exit"))

	ts.Shutdown()
}
