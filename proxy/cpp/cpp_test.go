package cpp_test

import (
	"flag"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var prewarm bool = false

func init() {
	flag.BoolVar(&prewarm, "prewarm", false, "Pre-warm the CPP proc")
}

func runSpawnLatency(ts *test.RealmTstate, kernels []string, evict bool, ncore proc.Tmcpu) *proc.Proc {
	args := []string{}
	if evict {
		args = append(args, "waitEvict")
	}
	p := proc.NewProc("spawn-latency-cpp", args)
	p.SetMcpu(ncore)
	start1 := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.Ts.T, err, "Start")
	SLEEP := 2 * time.Second
	start := time.Now()
	var evicted bool
	if evict {
		go func() {
			time.Sleep(SLEEP)
			evicted = true
			err := ts.Evict(p.GetPid())
			assert.Nil(ts.Ts.T, err, "Evict")
		}()
	}
	db.DPrintf(db.TEST, "CPP proc started (lat=%v)", time.Since(start1))
	status, err := ts.WaitExit(p.GetPid())
	db.DPrintf(db.TEST, "CPP proc exited, status: %v", status)
	assert.Nil(ts.Ts.T, err, "WaitExit error")
	if evict {
		assert.True(ts.Ts.T, evicted && time.Since(start) >= SLEEP, "Exited too fast %v %v", evicted, time.Since(start))
		assert.True(ts.Ts.T, status != nil && status.IsStatusEvicted(), "Exit status wrong: %v", status)
	} else {
		assert.True(ts.Ts.T, status != nil && status.IsStatusOK(), "Exit status wrong: %v", status)
	}
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
	p := runSpawnLatency(mrts.GetRealm(test.REALM1), nil, false, 0)
	db.DPrintf(db.TEST, "Proc done")

	b, err := mrts.GetRealm(test.REALM1).GetFile(filepath.Join(sp.S3, sp.LOCAL, "9ps3/hello-cpp-1"))
	assert.Nil(mrts.T, err, "Err GetFile: %v", err)
	assert.True(mrts.T, strings.Contains(string(b), p.GetPid().String()), "Proc output not in file")
}

func TestSpawnWaitEvict(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	db.DPrintf(db.TEST, "Running proc")
	runSpawnLatency(mrts.GetRealm(test.REALM1), nil, true, 0)
	db.DPrintf(db.TEST, "Proc done")
}

func TestSpawnLatency(t *testing.T) {
	const (
		N_PROC = 15
		N_NODE = 8
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	if err := mrts.GetRealm(test.REALM1).BootNode(N_NODE); !assert.Nil(t, err, "Err boot: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Running procs")
	c := make(chan bool)
	for i := 0; i < N_PROC; i++ {
		go func(c chan bool) {
			runSpawnLatency(mrts.GetRealm(test.REALM1), nil, false, 2000)
			c <- true
		}(c)
	}
	for i := 0; i < N_PROC; i++ {
		<-c
	}
	db.DPrintf(db.TEST, "Procs done")
}
