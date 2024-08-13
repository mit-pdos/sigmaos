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

	err = os.Remove("/tmp/sigmaos-perf/log.txt")
	assert.Nil(t, err)

	chkptProc := proc.NewProc("ckpt-proc", []string{"30"})
	err = ts.Spawn(chkptProc)
	assert.Nil(t, err)
	//err = ts.WaitStart(chkptProc.GetPid())
	//assert.Nil(t, err)

	time.Sleep(5 * time.Second)

	// pn := sp.S3 + "~any/fkaashoek/" + chkptProc.GetPid().String() + "/"
	pn := sp.UX + "~any/" + chkptProc.GetPid().String() + "/"

	db.DPrintf(db.TEST, "checkpointing %q", pn)
	osPid, err := ts.Checkpoint(chkptProc, pn)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "checkpoint pid: %d", osPid)

	time.Sleep(1 * time.Second)

	// spawn and run checkpointed proc
	restProc := proc.NewRestoreProc(chkptProc, pn, osPid)
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "sleep for a while")
	time.Sleep(30 * time.Second)

	//status, err := ts.WaitExit(restProc.GetPid())
	//assert.Nil(t, err)
	//assert.True(t, status.IsStatusOK())

	b, err := os.ReadFile("/tmp/sigmaos-perf/log.txt")
	db.DPrintf(db.TEST, "b %v\n", string(b))
	assert.True(t, strings.Contains(string(b), "........."))
	assert.True(t, strings.Contains(string(b), "exit"))

	ts.Shutdown()
}
