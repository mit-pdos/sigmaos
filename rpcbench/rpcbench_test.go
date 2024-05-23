package rpcbench_test

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rpcbench/proto"
	"sigmaos/rpcclnt"
	"sigmaos/test"
)

const (
	PATH     = "name/rpcbenchsrv"
	N_TRIALS = 50_000
)

func TestRPCPerf(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p := proc.NewProc("rpcbenchsrv", []string{PATH, strconv.FormatBool(test.Overlays)})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart")

	db.DPrintf(db.TEST, "Start RPCs")

	arg := proto.NoOpRequest{}
	res := proto.NoOpResult{}

	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, PATH)
	assert.Nil(t, err, "NewRPCClnt: %v", err)
	start := time.Now()
	for i := 0; i < N_TRIALS; i++ {
		err = rpcc.RPC("Srv.NoOp", &arg, &res)
		assert.Nil(t, err, "RPC: %v", err)
	}
	db.DPrintf(db.TEST, "Done RPCs")
	db.DPrintf(db.BENCH, "Average request latency: %v", time.Since(start)/N_TRIALS)

	err = ts.Evict(p.GetPid())
	assert.Nil(t, err, "Evict")
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	ts.Shutdown()
}

func TestCreateFilePerf(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p := proc.NewProc("rpcbenchsrv", []string{PATH, strconv.FormatBool(test.Overlays)})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart")

	db.DPrintf(db.TEST, "Start RPCs")

	start := time.Now()
	for i := 0; i < N_TRIALS; i++ {
		_, err = ts.Create(filepath.Join(PATH, strconv.Itoa(i)), 0777, 0)
		assert.Nil(t, err, "RPC: %v", err)
	}
	db.DPrintf(db.TEST, "Done RPCs")
	db.DPrintf(db.BENCH, "Average request latency: %v", time.Since(start)/N_TRIALS)

	err = ts.Evict(p.GetPid())
	assert.Nil(t, err, "Evict")
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	ts.Shutdown()
}

func TestMkDirPerf(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p := proc.NewProc("rpcbenchsrv", []string{PATH, strconv.FormatBool(test.Overlays)})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart")

	db.DPrintf(db.TEST, "Start RPCs")

	start := time.Now()
	for i := 0; i < N_TRIALS; i++ {
		err = ts.MkDir(filepath.Join(PATH, strconv.Itoa(i)), 0777)
		assert.Nil(t, err, "RPC: %v", err)
	}
	db.DPrintf(db.TEST, "Done RPCs")
	db.DPrintf(db.BENCH, "Average request latency: %v", time.Since(start)/N_TRIALS)

	err = ts.Evict(p.GetPid())
	assert.Nil(t, err, "Evict")
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	ts.Shutdown()
}

func TestMkDir1DeepPerf(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p := proc.NewProc("rpcbenchsrv", []string{PATH, strconv.FormatBool(test.Overlays)})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart")

	err = ts.MkDir(filepath.Join(PATH, "dir"), 0777)
	assert.Nil(t, err, "RPC: %v", err)

	db.DPrintf(db.TEST, "Start RPCs")

	start := time.Now()
	for i := 0; i < N_TRIALS; i++ {
		err = ts.MkDir(filepath.Join(PATH, "dir", strconv.Itoa(i)), 0777)
		assert.Nil(t, err, "RPC: %v", err)
	}
	db.DPrintf(db.TEST, "Done RPCs")
	db.DPrintf(db.BENCH, "Average request latency: %v", time.Since(start)/N_TRIALS)

	err = ts.Evict(p.GetPid())
	assert.Nil(t, err, "Evict")
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	ts.Shutdown()
}

func TestMkDir2DeepPerf(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p := proc.NewProc("rpcbenchsrv", []string{PATH, strconv.FormatBool(test.Overlays)})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart")

	err = ts.MkDir(filepath.Join(PATH, "dir"), 0777)
	assert.Nil(t, err, "RPC: %v", err)

	err = ts.MkDir(filepath.Join(PATH, "dir", "dir"), 0777)
	assert.Nil(t, err, "RPC: %v", err)

	db.DPrintf(db.TEST, "Start RPCs")

	start := time.Now()
	for i := 0; i < N_TRIALS; i++ {
		err = ts.MkDir(filepath.Join(PATH, "dir", "dir", strconv.Itoa(i)), 0777)
		assert.Nil(t, err, "RPC: %v", err)
	}
	db.DPrintf(db.TEST, "Done RPCs")
	db.DPrintf(db.BENCH, "Average request latency: %v", time.Since(start)/N_TRIALS)

	err = ts.Evict(p.GetPid())
	assert.Nil(t, err, "Evict")
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	ts.Shutdown()
}

func TestMkDir3DeepPerf(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	p := proc.NewProc("rpcbenchsrv", []string{PATH, strconv.FormatBool(test.Overlays)})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart")

	err = ts.MkDir(filepath.Join(PATH, "dir"), 0777)
	assert.Nil(t, err, "RPC: %v", err)

	err = ts.MkDir(filepath.Join(PATH, "dir", "dir"), 0777)
	assert.Nil(t, err, "RPC: %v", err)

	err = ts.MkDir(filepath.Join(PATH, "dir", "dir", "dir"), 0777)
	assert.Nil(t, err, "RPC: %v", err)

	db.DPrintf(db.TEST, "Start RPCs")

	start := time.Now()
	for i := 0; i < N_TRIALS; i++ {
		err = ts.MkDir(filepath.Join(PATH, "dir", "dir", "dir", strconv.Itoa(i)), 0777)
		assert.Nil(t, err, "RPC: %v", err)
	}
	db.DPrintf(db.TEST, "Done RPCs")
	db.DPrintf(db.BENCH, "Average request latency: %v", time.Since(start)/N_TRIALS)

	err = ts.Evict(p.GetPid())
	assert.Nil(t, err, "Evict")
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	ts.Shutdown()
}
