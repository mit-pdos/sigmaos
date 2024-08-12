package ckpt_test

import (
	"log"
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

	restProc := proc.NewRestoreProc(chkptProc, pn, osPid)

	// spawn and run it
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	log.Printf("spawned %v", err)

	time.Sleep(5 * time.Second)
	log.Printf("wait exit")
	status, err := ts.WaitExit(restProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	ts.Shutdown()
}
