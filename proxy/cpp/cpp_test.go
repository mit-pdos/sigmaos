package cpp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func runSpawnLatency(ts *test.RealmTstate, kernels []string) *proc.Proc {
	p := proc.NewProc("spawn-latency-cpp", []string{})
	err := ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.Ts.T, err, "Start")
	db.DPrintf(db.TEST, "CPP proc started")
	status, err := ts.WaitExit(p.GetPid())
	db.DPrintf(db.TEST, "CPP proc exited")
	assert.Nil(ts.Ts.T, err, "WaitExit error")
	assert.True(ts.Ts.T, status != nil && status.IsStatusOK(), "Exit status wrong: %v", status)
	return p
}

func TestCompile(t *testing.T) {
}

func TestSpawnWaitExit(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	db.DPrintf(db.TEST, "Running proc")
	runSpawnLatency(mrts.GetRealm(test.REALM1), nil)
	db.DPrintf(db.TEST, "Proc done")
}
