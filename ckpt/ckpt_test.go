package ckpt_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/hotel"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	rd "sigmaos/util/rand"
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
	db.DPrintf(db.TEST, "pid %v", pid)
	status, err := ts.WaitExit(ckptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusErr())

	pid = sp.GenPid("ckpt-proc-copy")

	db.DPrintf(db.TEST, "Spawn from checkpoint %v", pid)

	restProc := proc.NewProcFromCheckpoint(pid, PROGRAM+"-copy", pn)
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

	ckptProc := proc.NewProcPid(pid, GEO, []string{job, pn, "1000", "10", "20"})
	err = ts.Spawn(ckptProc)
	assert.Nil(t, err)
	err = ts.WaitStart(ckptProc.GetPid())
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Wait until proc %v has checkpointed itself", ckptProc.GetPid())

	status, err := ts.WaitExit(ckptProc.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusErr())
	//time.Sleep(100 * time.Millisecond)

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
