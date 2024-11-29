package ckpt_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/hotel"
	"sigmaos/proc"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	NPAGES  = "1000"
	PROGRAM = "ckpt-proc"
	GEO     = "hotel-geod"
	RUN     = 5
)

// Geo constants
const (
	DEF_GEO_N_IDX         = 1000
	DEF_GEO_SEARCH_RADIUS = 10
	DEF_GEO_N_RESULTS     = 5
)

func TestSpawnCkptProc(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	pid := sp.GenPid(PROGRAM)
	pn := sp.UX + "~any/" + pid.String() + "/"

	ckptProc := proc.NewProcPid(pid, PROGRAM, []string{strconv.Itoa(RUN), NPAGES, pn})
	err = ts.Spawn(ckptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(ckptProc.GetPid())
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until proc %v has checkpointed itself", ckptProc.GetPid())

	status, err := ts.WaitExit(ckptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusErr())

	time.Sleep(1000 * time.Millisecond)

	pid = sp.GenPid("ckpt-proc-copy")

	db.DPrintf(db.TEST, "Spawn from checkpoint %v", pid)

	restProc := proc.NewProcFromCheckpoint(pid, GEO+"-copy", pn)
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until start %v", pid)

	err = ts.WaitStart(restProc.GetPid())
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Started %v", restProc.GetPid())

	ts.Shutdown()
}

func TestSpawnCkptGeo(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	pid := sp.GenPid(GEO)
	pn := sp.UX + "~any/" + pid.String() + "/"

	job := rd.String(8)
	err = hotel.InitHotelFs(ts.FsLib, job)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Spawn proc %v %v", job, pn)

	ckptProc := proc.NewProcPid(pid, GEO, []string{job, pn, []string{"1000", "10", "20"}})
	err = ts.Spawn(ckptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(ckptProc.GetPid())
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until proc %v has checkpointed itself", ckptProc.GetPid())

	status, err := ts.WaitExit(ckptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusErr())

	time.Sleep(5 * time.Second)

	pid = sp.GenPid(GEO + "-copy")

	db.DPrintf(db.TEST, "Spawn from checkpoint %v", pid)

	restProc := proc.NewProcFromCheckpoint(pid, GEO+"-copy", pn)
	err = ts.Spawn(restProc)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until start %v", pid)

	err = ts.WaitStart(restProc.GetPid())
	assert.Nil(t, err)

	ts.Shutdown()
}
